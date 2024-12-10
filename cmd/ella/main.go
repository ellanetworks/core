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

func startNetwork(cfg config.Config) error {
	err := nms.Start(cfg.Api.Port, cfg.Api.TLS.Cert, cfg.Api.TLS.Key)
	if err != nil {
		return err
	}
	err = smf.Start()
	if err != nil {
		return err
	}
	err = amf.Start()
	if err != nil {
		return err
	}
	err = ausf.Start()
	if err != nil {
		return err
	}
	err = pcf.Start()
	if err != nil {
		return err
	}
	err = udr.Start()
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
	err = upf.Start(cfg.UPF.Interfaces, cfg.UPF.N3Address)
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
	level, err := zapcore.ParseLevel("debug")
	if err != nil {
		log.Fatalf("failed to parse log level: %v", err)
	}
	logger.SetLogLevel(level)
	err = db.Initialize(cfg.DB.Url, cfg.DB.Name)
	if err != nil {
		log.Fatalf("failed to initialize db: %v", err)
	}
	err = startNetwork(cfg)
	if err != nil {
		panic(err)
	}
	select {}
}
