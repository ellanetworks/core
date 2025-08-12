package runtime

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/api"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/pcf"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/tracing"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/internal/upf"
	"github.com/ellanetworks/core/version"
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

	if cfg.Telemetry.Enabled {
		tp, err := tracing.InitTracer(ctx, tracing.TelemetryConfig{
			OTLPEndpoint:   cfg.Telemetry.OTLPEndpoint,
			ServiceName:    "ella-core",
			ServiceVersion: version.GetVersion(),
		})
		if err != nil {
			return fmt.Errorf("couldn't initialize tracer: %w", err)
		}
		defer func() {
			if err := tp.Shutdown(ctx); err != nil {
				logger.EllaLog.Error("could not shutdown tracer", zap.Error(err))
			}
		}()
	}

	dbInstance, err := db.NewDatabase(cfg.DB.Path)
	if err != nil {
		return fmt.Errorf("couldn't initialize database: %w", err)
	}
	defer dbInstance.Close()

	auditWriter := dbInstance.AuditWriteFunc(ctx)

	logger.SetAuditDBWriter(auditWriter)

	metrics.RegisterDatabaseMetrics(dbInstance)

	scheme := api.HTTPS
	if cfg.Interfaces.API.TLS.Cert == "" || cfg.Interfaces.API.TLS.Key == "" {
		scheme = api.HTTP
	}

	if err := api.Start(
		dbInstance,
		cfg.Interfaces.API.Port,
		scheme,
		cfg.Interfaces.API.TLS.Cert,
		cfg.Interfaces.API.TLS.Key,
		cfg.Interfaces.N3.Name,
		cfg.Interfaces.N6.Name,
		cfg.Telemetry.Enabled,
		rc.EmbedFS,
		rc.RegisterExtraRoutes,
	); err != nil {
		return fmt.Errorf("couldn't start API: %w", err)
	}

	if err := smf.Start(dbInstance); err != nil {
		return fmt.Errorf("couldn't start SMF: %w", err)
	}
	if err := amf.Start(ctx, dbInstance, cfg.Interfaces.N2.Address, cfg.Interfaces.N2.Port); err != nil {
		return fmt.Errorf("couldn't start AMF: %w", err)
	}
	if err := pcf.Start(dbInstance); err != nil {
		return fmt.Errorf("couldn't start PCF: %w", err)
	}
	if err := udm.Start(dbInstance); err != nil {
		return fmt.Errorf("couldn't start UDM: %w", err)
	}
	upfInstance, err := upf.Start(ctx, cfg.Interfaces.N3.Address, cfg.Interfaces.N3.Name, cfg.Interfaces.N6.Name, cfg.XDP.AttachMode)
	if err != nil {
		return fmt.Errorf("couldn't start UPF: %w", err)
	}
	defer upfInstance.Close()
	defer amf.Close()

	<-ctx.Done()
	logger.EllaLog.Info("Shutdown signal received, exiting.")
	return nil
}
