package runtime

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/api"
	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/jobs"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pfcp_dispatcher"
	"github.com/ellanetworks/core/internal/sessions"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	smf_pfcp "github.com/ellanetworks/core/internal/smf/pfcp"
	"github.com/ellanetworks/core/internal/tracing"
	"github.com/ellanetworks/core/internal/upf"
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

	logger.SetDb(dbInstance)

	jobs.StartDataRetentionWorker(dbInstance)

	go sessions.CleanUp(ctx, dbInstance)

	isNATEnabled, err := dbInstance.IsNATEnabled(ctx)
	if err != nil {
		return fmt.Errorf("couldn't determine if NAT is enabled: %w", err)
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

	upfInstance, err := upf.Start(ctx, cfg.Interfaces.N3, n3Address, advertisedN3Address, cfg.Interfaces.N6, cfg.XDP.AttachMode, isNATEnabled)
	if err != nil {
		return fmt.Errorf("couldn't start UPF: %w", err)
	}

	if err := api.Start(
		dbInstance,
		cfg,
		upfInstance,
		rc.EmbedFS,
		rc.RegisterExtraRoutes,
	); err != nil {
		return fmt.Errorf("couldn't start API: %w", err)
	}

	smf.Start(dbInstance)

	if err := amf.Start(ctx, dbInstance, cfg.Interfaces.N2.Address, cfg.Interfaces.N2.Port, &pdusession.EllaSmfSbi{}); err != nil {
		return fmt.Errorf("couldn't start AMF: %w", err)
	}

	ausf.Start(dbInstance)

	server.RegisterMetrics()
	amf.RegisterMetrics()
	smf.RegisterMetrics()
	upf.RegisterMetrics()

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		amf.Close(shutdownCtx)
		upfInstance.Close()

		err := dbInstance.Close()
		if err != nil {
			logger.EllaLog.Error("couldn't close database", zap.Error(err))
		}

		if tp == nil {
			return
		}

		err = tp.Shutdown(shutdownCtx)
		if err != nil {
			logger.EllaLog.Error("could not shutdown tracer", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.EllaLog.Info("Shutdown signal received, exiting.")

	return nil
}
