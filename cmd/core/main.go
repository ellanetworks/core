// Copyright 2024 Ella Networks

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/api"
	"github.com/ellanetworks/core/internal/ausf"
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

const (
	InitialMcc         = "001"
	InitialMnc         = "01"
	InitialOperatorSst = 1
	InitialOperatorSd  = 1056816
)

func startNetwork(dbInstance *db.Database, cfg config.Config) error {
	scheme := api.HTTPS
	if cfg.Interfaces.API.TLS.Cert == "" || cfg.Interfaces.API.TLS.Key == "" {
		scheme = api.HTTP
	}

	err := api.Start(dbInstance, cfg.Interfaces.API.Port, scheme, cfg.Interfaces.API.TLS.Cert, cfg.Interfaces.API.TLS.Key, cfg.Interfaces.N3.Name, cfg.Interfaces.N6.Name, cfg.Telemetry.Enabled)
	if err != nil {
		return err
	}

	err = smf.Start(dbInstance)
	if err != nil {
		return err
	}

	err = amf.Start(dbInstance, cfg.Interfaces.N2.Address, cfg.Interfaces.N2.Port)
	if err != nil {
		return err
	}

	err = ausf.Start()
	if err != nil {
		return err
	}

	err = pcf.Start(dbInstance)
	if err != nil {
		return err
	}

	err = udm.Start(dbInstance)
	if err != nil {
		return err
	}

	err = upf.Start(cfg.Interfaces.N3.Address, cfg.Interfaces.N3.Name, cfg.Interfaces.N6.Name, cfg.XDP.AttachMode)
	if err != nil {
		return fmt.Errorf("failed to start UPF: %v", err)
	}

	return nil
}

func main() {
	log.SetOutput(os.Stderr)

	configFilePtr := flag.String("config", "", "The config file to be provided to the server")

	flag.Parse()

	if *configFilePtr == "" {
		log.Fatalf("No config file provided. Use `-config` to provide a config file")
	}

	cfg, err := config.Validate(*configFilePtr)
	if err != nil {
		log.Fatalf("Couldn't validate config file: %s", err)
	}

	err = logger.ConfigureLogging(cfg.Logging.SystemLogging.Level, cfg.Logging.SystemLogging.Output, cfg.Logging.SystemLogging.Path, cfg.Logging.AuditLogging.Output, cfg.Logging.AuditLogging.Path)
	if err != nil {
		log.Fatalf("Failed to configure logging: %v", err)
	}

	version := version.GetVersion()

	if cfg.Telemetry.Enabled {
		ctx := context.Background()
		tp, err := tracing.InitTracer(ctx, tracing.TelemetryConfig{
			OTLPEndpoint:   cfg.Telemetry.OTLPEndpoint,
			ServiceName:    "ella-core",
			ServiceVersion: version,
		})
		if err != nil {
			logger.EllaLog.Panic("could not initialize tracer", zap.Error(err))
		}

		defer func() {
			err := tp.Shutdown(ctx)
			if err != nil {
				logger.EllaLog.Error("failed to shutdown tracer", zap.Error(err))
			}
		}()
	}

	initialOp, err := generateOperatorCode()
	if err != nil {
		logger.EllaLog.Panic("Failed to generate operator code", zap.Error(err))
	}

	initialHNPrivateKey, err := generateHomeNetworkPrivateKey()
	if err != nil {
		logger.EllaLog.Panic("Failed to generate home network private key", zap.Error(err))
	}

	initialOperatorValues := db.Operator{
		Mcc:                   InitialMcc,
		Mnc:                   InitialMnc,
		OperatorCode:          initialOp,
		Sst:                   InitialOperatorSst,
		Sd:                    InitialOperatorSd,
		HomeNetworkPrivateKey: initialHNPrivateKey,
	}
	initialOperatorValues.SetSupportedTacs(
		[]string{
			"001",
		},
	)

	dbInstance, err := db.NewDatabase(cfg.DB.Path, initialOperatorValues)
	if err != nil {
		logger.EllaLog.Panic("Failed to initialize database", zap.Error(err))
	}

	defer func() {
		if err := dbInstance.Close(); err != nil {
			logger.EllaLog.Panic("Failed to close database", zap.Error(err))
		}
	}()

	metrics.RegisterDatabaseMetrics(dbInstance)

	err = startNetwork(dbInstance, cfg)
	if err != nil {
		logger.EllaLog.Panic("Failed to start network", zap.Error(err))
	}

	logger.EllaLog.Info("Ella Core is running", zap.String("version", version))

	select {}
}

func generateOperatorCode() (string, error) {
	var op [16]byte
	_, err := rand.Read(op[:])
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(op[:]), nil
}

func generateHomeNetworkPrivateKey() (string, error) {
	var privateKey [32]byte
	_, err := rand.Read(privateKey[:])
	if err != nil {
		return "", fmt.Errorf("failed to generate home network private key: %w", err)
	}

	// Ensure the private key conforms to Curve25519 requirements:
	privateKey[0] &= 248 // Clamp first byte
	privateKey[31] &= 127
	privateKey[31] |= 64 // Set highest bit

	return hex.EncodeToString(privateKey[:]), nil
}
