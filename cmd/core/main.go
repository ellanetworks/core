package main

import (
	"flag"
	"log"
	"os"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/nms"
	"github.com/ellanetworks/core/internal/nssf"
	"github.com/ellanetworks/core/internal/pcf"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/internal/udr"
	"github.com/ellanetworks/core/internal/upf"
	"go.uber.org/zap/zapcore"
)

func startNetwork(dbInstance *db.Database, cfg config.Config) error {
	err := nms.Start(dbInstance, cfg.Interfaces.API.Port, cfg.Interfaces.API.TLS.Cert, cfg.Interfaces.API.TLS.Key)
	if err != nil {
		return err
	}
	err = smf.Start(dbInstance)
	if err != nil {
		return err
	}
	err = amf.Start(dbInstance)
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
	err = udr.Start(dbInstance)
	if err != nil {
		return err
	}
	err = udm.Start()
	if err != nil {
		return err
	}
	err = nssf.Start()
	if err != nil {
		return err
	}
	err = upf.Start(cfg.Interfaces.N3.Address, cfg.Interfaces.N3.Name, cfg.Interfaces.N6.Name)
	if err != nil {
		return err
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
	level, err := zapcore.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatalf("failed to parse log level: %v", err)
	}
	logger.SetLogLevel(level)
	dbInstance, err := db.NewDatabase(cfg.DB.Path)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := dbInstance.Close(); err != nil {
			log.Fatalf("Failed to close database: %v", err)
		}
	}()
	metrics.RegisterDatabaseMetrics(dbInstance)
	err = startNetwork(dbInstance, cfg)
	if err != nil {
		logger.EllaLog.Panicf("Failed to start network: %v", err)
	}
	logger.EllaLog.Infof("Ella is running")
	select {}
}
