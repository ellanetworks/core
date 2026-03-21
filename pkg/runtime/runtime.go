package runtime

import (
	"archive/tar"
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	amfcontext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/api"
	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
	"github.com/ellanetworks/core/internal/jobs"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/ellanetworks/core/internal/sessions"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	smf_pfcp "github.com/ellanetworks/core/internal/smf/pfcp"
	"github.com/ellanetworks/core/internal/supportbundle"
	"github.com/ellanetworks/core/internal/tracing"
	"github.com/ellanetworks/core/internal/upf"
	"github.com/ellanetworks/core/internal/upf/bpfdump"
	upf_pfcp "github.com/ellanetworks/core/internal/upf/core"
	"github.com/ellanetworks/core/version"
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

	jobs.StartDataRetentionWorker(ctx, dbInstance)

	go sessions.CleanUp(ctx, dbInstance)

	isNATEnabled, err := dbInstance.IsNATEnabled(ctx)
	if err != nil {
		return fmt.Errorf("couldn't determine if NAT is enabled: %w", err)
	}

	isFlowAccountingEnabled, err := dbInstance.IsFlowAccountingEnabled(ctx)
	if err != nil {
		return fmt.Errorf("couldn't determine if flow accounting is enabled: %w", err)
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

	pfcp_dispatcher.Dispatcher = pfcp_dispatcher.NewPfcpDispatcher(smf_pfcp.SmfPfcpHandler{}, upf_pfcp.UpfPfcpHandler{})

	upfInstance, err := upf.Start(ctx, cfg.Interfaces.N3, n3Address, advertisedN3Address, cfg.Interfaces.N6, cfg.XDP.AttachMode, isNATEnabled, isFlowAccountingEnabled)
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
	amf.RegisterMetrics()
	smf.RegisterMetrics()
	upf.RegisterMetrics()

	apiServer, err := api.Start(
		ctx,
		dbInstance,
		cfg,
		upfInstance,
		rc.EmbedFS,
		rc.RegisterExtraRoutes,
	)
	if err != nil {
		return fmt.Errorf("couldn't start API: %w", err)
	}

	smf.Start(dbInstance)
	ausf.Start(ctx, dbInstance)

	sctpServer, err := amf.Start(ctx, dbInstance, cfg.Interfaces.N2.Address, cfg.Interfaces.N2.Port, &pdusession.EllaSmfSbi{})
	if err != nil {
		return fmt.Errorf("couldn't start AMF: %w", err)
	}

	supportbundle.AMFDumper = func(ctx context.Context) (any, error) {
		return amfcontext.ExportUEs(ctx)
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		logger.EllaLog.Info("Shutting down API server")

		if err := apiServer.Shutdown(shutdownCtx); err != nil {
			logger.EllaLog.Error("API server shutdown error", zap.Error(err))
		}

		logger.EllaLog.Info("Shutting down AMF")
		amf.Close(shutdownCtx, sctpServer)

		logger.EllaLog.Info("Shutting down UPF")
		upfInstance.Close(shutdownCtx)

		logger.EllaLog.Info("Flushing buffered writer")
		bufferedWriter.Stop(shutdownCtx)

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
