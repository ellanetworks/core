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
	"github.com/ellanetworks/core/internal/ipam"
	"github.com/ellanetworks/core/internal/jobs"
	"github.com/ellanetworks/core/internal/kernel"
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

	err = dbInstance.DeleteAllDynamicLeases(ctx)
	if err != nil {
		return fmt.Errorf("couldn't release all dynamic leases: %w", err)
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

	realKernel := kernel.NewRealKernel(cfg.Interfaces.N3.Name, cfg.Interfaces.N6.Name)

	uePools := collectUEPools(ctx, dbInstance)
	routeFilter := bgp.BuildRouteFilter(uePools, net.ParseIP(cfg.Interfaces.N3.Address), cfg.Interfaces.N6.Name)
	importStore := &bgpImportPrefixAdapter{db: dbInstance}

	bgpService := bgp.New(net.ParseIP(n6IP), logger.EllaLog,
		bgp.WithKernel(realKernel),
		bgp.WithImportPrefixStore(importStore),
		bgp.WithRouteFilter(routeFilter),
	)

	bgpSettings, err := dbInstance.GetBGPSettings(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get BGP settings: %w", err)
	}

	if bgpSettings.Enabled {
		bgpPeers, err := dbInstance.ListAllBGPPeers(ctx)
		if err != nil {
			return fmt.Errorf("couldn't list BGP peers: %w", err)
		}

		activeLeases, err := dbInstance.ListActiveLeases(ctx)
		if err != nil {
			return fmt.Errorf("couldn't list active leases: %w", err)
		}

		allocatedIPs := make(map[string]string, len(activeLeases))
		for _, l := range activeLeases {
			allocatedIPs[l.Address().String()] = l.IMSI
		}

		servicePeers := server.DBPeersToBGPPeers(bgpPeers)

		err = bgpService.Start(ctx, server.DBSettingsToBGPSettings(bgpSettings), servicePeers, allocatedIPs, !isNATEnabled)
		if err != nil {
			listenAddr := bgpSettings.ListenAddress
			if listenAddr == "" {
				listenAddr = ":179"
			}

			logger.EllaLog.Error("BGP failed to start: address may be in use. Stop any external BGP daemon (FRR, BIRD) before enabling integrated BGP.", zap.String("address", listenAddr), zap.Error(err))
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
	smfPCF := &pcfDBAdapter{db: dbInstance}
	smfStore := &smfDBAdapter{db: dbInstance, allocator: ipam.NewSequentialAllocator(&leaseStoreAdapter{db: dbInstance})}
	smfAMF := &smfAMFAdapter{}

	smfInstance := smf.New(smfPCF, smfStore, nil, smfAMF, smf.WithNodeID(net.ParseIP(n3Address)), smf.WithBGP(bgpService))

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

	// Initialize SDF filters from database
	conn := upf_pfcp.GetConnection()
	if conn != nil && dbInstance != nil {
		conn.SetBPFObjects(conn.BpfObjects, dbInstance)
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
		// Each shutdown step gets its own timeout so that a slow step
		// does not starve subsequent ones.
		stepTimeout := 5 * time.Second

		// 1. Stop accepting new HTTP requests.
		logger.EllaLog.Info("Shutting down API server")

		apiCtx, apiCancel := context.WithTimeout(context.Background(), stepTimeout)
		if err := apiServer.Shutdown(apiCtx); err != nil {
			logger.EllaLog.Warn("API server shutdown error", zap.Error(err))
		}

		apiCancel()

		// 2. Cancel all AMF UE timers immediately so paging and other
		//    retransmissions stop firing during teardown.
		logger.EllaLog.Info("Cancelling AMF timers")
		amfInstance.StopAllTimers()

		// 3. Notify RANs and close SCTP connections.
		logger.EllaLog.Info("Shutting down AMF")

		amfCtx, amfCancel := context.WithTimeout(context.Background(), stepTimeout)
		closeAMF(amfCtx, amfInstance, sctpServer)
		amfCancel()

		// 4. Stop BGP (no context needed, returns synchronously).
		logger.EllaLog.Info("Shutting down BGP")

		if err := bgpService.Stop(); err != nil {
			logger.EllaLog.Error("BGP service shutdown error", zap.Error(err))
		}

		// 5. Stop UPF — this flushes remaining flow reports to SMF.
		logger.EllaLog.Info("Shutting down UPF")

		upfCtx, upfCancel := context.WithTimeout(context.Background(), stepTimeout)
		upfInstance.Close(upfCtx)
		upfCancel()

		// 6. Drain the buffered writer so queued events (including the
		//    flow reports just flushed by the UPF) are persisted to DB.
		logger.EllaLog.Info("Flushing buffered writer")

		bwCtx, bwCancel := context.WithTimeout(context.Background(), stepTimeout)
		bufferedWriter.Stop(bwCtx)
		bwCancel()

		// 7. Wait for background goroutines (data retention, session
		//    cleanup, AUSF) which were already signalled via ctx.Done().
		logger.EllaLog.Info("Waiting for background goroutines")
		wg.Wait()

		// 8. Close the database now that all writers have drained.
		logger.EllaLog.Info("Closing database")

		if err := dbInstance.Close(); err != nil {
			logger.EllaLog.Error("couldn't close database", zap.Error(err))
		}

		// 9. Flush the OpenTelemetry tracer.
		if tp != nil {
			logger.EllaLog.Info("Shutting down tracer")

			tpCtx, tpCancel := context.WithTimeout(context.Background(), stepTimeout)
			if err := tp.Shutdown(tpCtx); err != nil {
				logger.EllaLog.Warn("could not shutdown tracer", zap.Error(err))
			}

			tpCancel()
		}
	}()

	<-ctx.Done()
	logger.EllaLog.Info("Shutdown signal received, exiting.")

	return nil
}

func closeAMF(ctx context.Context, amfInstance *amf.AMF, srv *service.Server) {
	// Use a short dedicated timeout for the DB query so it doesn't
	// consume the caller's full shutdown budget.
	queryCtx, queryCancel := context.WithTimeout(ctx, 2*time.Second)
	operatorInfo, err := amfInstance.GetOperatorInfo(queryCtx)

	queryCancel()

	if err != nil {
		logger.AmfLog.Error("Could not get operator info", zap.Error(err))
	} else {
		unavailableGuamiList := send.BuildUnavailableGUAMIList(operatorInfo.Guami)

		for _, ran := range amfInstance.ListRadios() {
			if err := ran.NGAPSender.SendAMFStatusIndication(ctx, unavailableGuamiList); err != nil {
				logger.AmfLog.Error("failed to send AMF Status Indication to RAN", zap.Error(err))
			}
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

// bgpImportPrefixAdapter adapts *db.Database to the bgp.ImportPrefixStore interface.
type bgpImportPrefixAdapter struct {
	db *db.Database
}

func (a *bgpImportPrefixAdapter) ListImportPrefixes(ctx context.Context, peerID int) ([]bgp.ImportPrefixEntry, error) {
	dbPrefixes, err := a.db.ListImportPrefixesByPeer(ctx, peerID)
	if err != nil {
		return nil, err
	}

	entries := make([]bgp.ImportPrefixEntry, len(dbPrefixes))
	for i, p := range dbPrefixes {
		entries[i] = bgp.ImportPrefixEntry{
			Prefix:    p.Prefix,
			MaxLength: p.MaxLength,
		}
	}

	return entries, nil
}

// collectUEPools returns the UE IP pool CIDRs from all data networks.
func collectUEPools(ctx context.Context, dbInstance *db.Database) []*net.IPNet {
	dataNetworks, err := dbInstance.ListAllDataNetworks(ctx)
	if err != nil {
		logger.EllaLog.Warn("failed to list data networks for BGP filter", zap.Error(err))

		return nil
	}

	var pools []*net.IPNet

	for _, dn := range dataNetworks {
		_, network, err := net.ParseCIDR(dn.IPPool)
		if err != nil {
			continue
		}

		pools = append(pools, network)
	}

	return pools
}
