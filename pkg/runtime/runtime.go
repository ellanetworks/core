package runtime

import (
	"archive/tar"
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/ngap/service"
	amfsctp "github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/api"
	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/jobs"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/ellanetworks/core/internal/sessions"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/supportbundle"
	"github.com/ellanetworks/core/internal/tracing"
	"github.com/ellanetworks/core/internal/upf"
	"github.com/ellanetworks/core/internal/upf/bpfdump"
	upf_pfcp "github.com/ellanetworks/core/internal/upf/core"
	"github.com/ellanetworks/core/version"
	nasLogger "github.com/free5gc/nas/logger"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

type RuntimeConfig struct {
	ConfigPath          string
	RegisterExtraRoutes func(mux *http.ServeMux)
	EmbedFS             fs.FS
}

func Start(ctx context.Context, rc RuntimeConfig) error {
	cfg, err := config.Validate(rc.ConfigPath)
	if err != nil {
		return fmt.Errorf("couldn't validate config: %w", err)
	}

	if err := logger.ConfigureLogging(
		cfg.Logging.SystemLogging.Level,
		cfg.Logging.SystemLogging.Output,
		cfg.Logging.SystemLogging.Path,
		cfg.Logging.AuditLogging.Output,
		cfg.Logging.AuditLogging.Path,
	); err != nil {
		return fmt.Errorf("couldn't configure logging: %w", err)
	}

	ver := version.GetVersion()

	logger.EllaLog.Info("Starting Ella Core",
		zap.String("version", ver.Version),
		zap.String("revision", ver.Revision),
	)

	var tp *trace.TracerProvider

	if cfg.Telemetry.Enabled {
		tp, err = tracing.InitTracer(ctx, tracing.TelemetryConfig{
			OTLPEndpoint:    cfg.Telemetry.OTLPEndpoint,
			ServiceName:     "ella-core",
			ServiceVersion:  ver.Version,
			ServiceRevision: ver.Revision,
		})
		if err != nil {
			return fmt.Errorf("couldn't initialize tracer: %w", err)
		}
	}

	dbInstance, err := db.NewDatabase(ctx, cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("couldn't initialize database: %w", err)
	}

	err = dbInstance.ReleaseAllIPs(ctx)
	if err != nil {
		return fmt.Errorf("couldn't release all IPs: %w", err)
	}

	bufferedWriter := dbwriter.NewBufferedDBWriter(dbInstance, 1000, logger.NetworkLog)
	logger.SetDb(bufferedWriter)

	// Provide the runtime config file contents to the supportbundle generator
	// so support bundles include the exact YAML used at startup. This is safe
	// because main controls what is exposed via the provider; here we simply
	// read the file from the configured path.
	supportbundle.ConfigProvider = func(ctx context.Context) ([]byte, error) {
		return os.ReadFile(rc.ConfigPath)
	}

	var wg sync.WaitGroup

	wg.Go(func() {
		jobs.RunDataRetentionWorker(ctx, dbInstance)
	})

	wg.Go(func() {
		sessions.CleanUp(ctx, dbInstance)
	})

	isNATEnabled, err := dbInstance.IsNATEnabled(ctx)
	if err != nil {
		return fmt.Errorf("couldn't determine if NAT is enabled: %w", err)
	}

	isFlowAccountingEnabled, err := dbInstance.IsFlowAccountingEnabled(ctx)
	if err != nil {
		return fmt.Errorf("couldn't determine if flow accounting is enabled: %w", err)
	}

	// Initialize BGP service
	n6IP, err := config.GetInterfaceIPFunc(cfg.Interfaces.N6.Name)
	if err != nil {
		return fmt.Errorf("couldn't get N6 interface IP: %w", err)
	}

	bgpService := bgp.New(net.ParseIP(n6IP), logger.EllaLog)

	bgpSettings, err := dbInstance.GetBGPSettings(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get BGP settings: %w", err)
	}

	if bgpSettings.Enabled {
		bgpPeers, err := dbInstance.ListAllBGPPeers(ctx)
		if err != nil {
			return fmt.Errorf("couldn't list BGP peers: %w", err)
		}

		allocatedIPs, err := dbInstance.ListAllocatedIPs(ctx)
		if err != nil {
			return fmt.Errorf("couldn't list allocated IPs: %w", err)
		}

		servicePeers := server.DBPeersToBGPPeers(bgpPeers)

		err = bgpService.Start(ctx, server.DBSettingsToBGPSettings(bgpSettings), servicePeers, allocatedIPs)
		if err != nil {
			logger.EllaLog.Error("BGP failed to start: port 179 may be in use. Stop any external BGP daemon (FRR, BIRD) before enabling integrated BGP.",
				zap.Error(err))
		}
	}

	n3Address := cfg.Interfaces.N3.Address

	n3Settings, err := dbInstance.GetN3Settings(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get N3 external address: %w", err)
	}

	advertisedN3Address := n3Address
	if n3Settings != nil && n3Settings.ExternalAddress != "" {
		advertisedN3Address = n3Settings.ExternalAddress
		logger.EllaLog.Debug("Using N3 external address from N3 settings", zap.String("n3_external_address", advertisedN3Address))
	}

	// Create SMF with dependency-injected adapters.
	smfStore := &smfDBAdapter{db: dbInstance}
	smfAMF := &smfAMFAdapter{}

	smfInstance := smf.New(smfStore, nil, smfAMF, smf.WithNodeID(net.ParseIP(n3Address)), smf.WithBGP(bgpService))

	// The SMF instance implements pfcp_dispatcher.SMF (HandlePfcpSessionReportRequest, SendFlowReport).
	dispatcher := pfcp_dispatcher.NewPfcpDispatcher(smfInstance, upf_pfcp.UpfPfcpHandler{})

	// Now that dispatcher is initialized, create the SMF UPF adapter with it
	smfUPF := &smfUPFAdapter{
		dispatcher: &dispatcher,
		nodeID:     net.ParseIP(n3Address),
	}
	smfInstance.SetUPF(smfUPF)

	upfInstance, err := upf.Start(ctx, &dispatcher, cfg.Interfaces.N3, n3Address, advertisedN3Address, cfg.Interfaces.N6, cfg.XDP.AttachMode, isNATEnabled, isFlowAccountingEnabled)
	if err != nil {
		return fmt.Errorf("couldn't start UPF: %w", err)
	}

	// Wire supportbundle BPF dumper to dump live BPF maps from the UPF process.
	// The closure captures the live BPF objects via the PFCP connection stored
	// in the UPF core package. If the UPF hasn't initialized BPF objects, the
	// dumper is a no-op.
	supportbundle.BpfDumper = func(ctx context.Context, tw *tar.Writer) error {
		conn := upf_pfcp.GetConnection()
		if conn == nil || conn.BpfObjects == nil {
			// graceful no-op
			return nil
		}

		opts := bpfdump.DumpOptions{
			Exclude:          []string{"nat_ct", "flow_stats", "nocp_map"},
			MaxEntriesPerMap: 10000,
		}

		_, err := bpfdump.DumpAll(ctx, conn.BpfObjects, opts, tw)
		if err != nil {
			logger.EllaLog.Error("supportbundle: bpf dump failed", zap.Error(err))
			return err
		}

		logger.EllaLog.Info("supportbundle: bpf dump completed")

		return nil
	}

	server.RegisterMetrics()
	smf.RegisterMetrics(smfInstance)
	upf.RegisterMetrics()

	ausfStore := &ausfDBAdapter{db: dbInstance}
	keyResolver := func(scheme string, keyID int) (string, error) {
		key, err := dbInstance.GetHomeNetworkKeyBySchemeAndIdentifier(ctx, scheme, keyID)
		if err != nil {
			return "", err
		}

		return key.PrivateKey, nil
	}

	ausfInstance := ausf.New(ausfStore, keyResolver)

	wg.Go(func() {
		ausfInstance.Run(ctx)
	})

	amfInstance := amf.New(dbInstance, ausfInstance, smfInstance)
	smfAMF.amf = amfInstance

	amf.RegisterMetrics(amfInstance)
	gmm.RegisterMetrics()
	ngap.RegisterMetrics()

	apiServer, err := api.Start(
		ctx,
		dbInstance,
		cfg,
		upfInstance,
		smfInstance,
		amfInstance,
		bgpService,
		rc.EmbedFS,
		rc.RegisterExtraRoutes,
	)
	if err != nil {
		return fmt.Errorf("couldn't start API: %w", err)
	}

	nasLogger.SetLogLevel(0) // Suppress free5gc NAS log output

	sctpServer := service.NewServer(service.Callbacks{
		Dispatch: func(ctx context.Context, conn *amfsctp.SCTPConn, msg []byte) {
			ngap.Dispatch(ctx, amfInstance, conn, msg)
		},
		Notify: func(conn *amfsctp.SCTPConn, notification amfsctp.Notification) {
			ngap.HandleSCTPNotification(amfInstance, conn, notification)
		},
		OnDisconnect: func(conn *amfsctp.SCTPConn) {
			if ran, ok := amfInstance.FindRadioByConn(conn); ok {
				amfInstance.RemoveRadio(ran)
				logger.AmfLog.Info("removed radio on connection close", zap.Int("fd", conn.Fd()))
			}
		},
	})

	err = sctpServer.ListenAndServe(ctx, cfg.Interfaces.N2.Address, cfg.Interfaces.N2.Port)
	if err != nil {
		return fmt.Errorf("couldn't start AMF: %w", err)
	}

	supportbundle.AMFDumper = func(ctx context.Context) (any, error) {
		return amfInstance.ExportUEs(ctx)
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		logger.EllaLog.Info("Shutting down API server")

		if err := apiServer.Shutdown(shutdownCtx); err != nil {
			logger.EllaLog.Error("API server shutdown error", zap.Error(err))
		}

		logger.EllaLog.Info("Shutting down AMF")
		closeAMF(shutdownCtx, amfInstance, sctpServer)

		logger.EllaLog.Info("Shutting down BGP")

		if err := bgpService.Stop(); err != nil {
			logger.EllaLog.Error("BGP service shutdown error", zap.Error(err))
		}

		logger.EllaLog.Info("Shutting down UPF")
		upfInstance.Close(shutdownCtx)

		logger.EllaLog.Info("Flushing buffered writer")
		bufferedWriter.Stop(shutdownCtx)

		logger.EllaLog.Info("Waiting for background goroutines")
		wg.Wait()

		logger.EllaLog.Info("Closing database")

		err := dbInstance.Close()
		if err != nil {
			logger.EllaLog.Error("couldn't close database", zap.Error(err))
		}

		if tp == nil {
			return
		}

		logger.EllaLog.Info("Shutting down tracer")

		err = tp.Shutdown(shutdownCtx)
		if err != nil {
			logger.EllaLog.Error("could not shutdown tracer", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.EllaLog.Info("Shutdown signal received, exiting.")

	return nil
}

func closeAMF(ctx context.Context, amfInstance *amf.AMF, srv *service.Server) {
	operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
	if err != nil {
		logger.AmfLog.Error("Could not get operator info", zap.Error(err))
		return
	}

	unavailableGuamiList := send.BuildUnavailableGUAMIList(operatorInfo.Guami)

	for _, ran := range amfInstance.ListRadios() {
		err := ran.NGAPSender.SendAMFStatusIndication(ctx, unavailableGuamiList)
		if err != nil {
			logger.AmfLog.Error("failed to send AMF Status Indication to RAN", zap.Error(err))
		}
	}

	srv.Shutdown(ctx)

	logger.AmfLog.Info("AMF terminated")
}

// ausfDBAdapter adapts *db.Database to the ausf.SubscriberStore interface.
type ausfDBAdapter struct {
	db *db.Database
}

func (a *ausfDBAdapter) GetSubscriber(ctx context.Context, imsi string) (*ausf.Subscriber, error) {
	sub, err := a.db.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, err
	}

	return &ausf.Subscriber{
		PermanentKey:   sub.PermanentKey,
		Opc:            sub.Opc,
		SequenceNumber: sub.SequenceNumber,
	}, nil
}

func (a *ausfDBAdapter) UpdateSequenceNumber(ctx context.Context, imsi string, sqn string) error {
	return a.db.EditSubscriberSequenceNumber(ctx, imsi, sqn)
}
