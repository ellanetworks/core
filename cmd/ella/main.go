package main

import (
	"flag"
	"log"
	"os"

	"github.com/yeastengine/ella/internal/amf"
	"github.com/yeastengine/ella/internal/ausf"
	"github.com/yeastengine/ella/internal/config"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/nms"
	"github.com/yeastengine/ella/internal/nssf"
	"github.com/yeastengine/ella/internal/pcf"
	"github.com/yeastengine/ella/internal/smf"
	"github.com/yeastengine/ella/internal/udm"
	"github.com/yeastengine/ella/internal/udr"
	"github.com/yeastengine/ella/internal/upf"
	"go.uber.org/zap/zapcore"
)

func startNetwork(dbInstance *db.Database, cfg config.Config) error {
	err := nms.Start(dbInstance, cfg.Interfaces.API.Port, cfg.Interfaces.API.TLS.Cert, cfg.Interfaces.API.TLS.Key)
	if err != nil {
		return err
	}
	err = smf.Start()
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
	defer dbInstance.Close()
	err = startNetwork(dbInstance, cfg)
	if err != nil {
		logger.EllaLog.Panicf("Failed to start network: %v", err)
	}
	logger.EllaLog.Infof("Ella is running")
	select {}
}
