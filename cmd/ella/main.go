package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/yeastengine/ella/internal/amf"
	"github.com/yeastengine/ella/internal/ausf"
	"github.com/yeastengine/ella/internal/config"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/nms"
	"github.com/yeastengine/ella/internal/nssf"
	"github.com/yeastengine/ella/internal/pcf"
	"github.com/yeastengine/ella/internal/smf"
	"github.com/yeastengine/ella/internal/udm"
	"github.com/yeastengine/ella/internal/udr"
	"github.com/yeastengine/ella/internal/upf"
)

func parseFlags() (config.Config, error) {
	flag.String("config", "", "/path/to/config.yaml")
	flag.Parse()
	configFile := flag.Lookup("config").Value.String()
	if configFile == "" {
		return config.Config{}, fmt.Errorf("config file not provided")
	}
	cfg, err := config.Parse(configFile)
	if err != nil {
		return config.Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}
	return cfg, nil
}

func startNetwork(cfg config.Config) error {
	amfUrl := "http://127.0.0.1:29518"
	udmUrl := "http://127.0.0.1:29503"
	_, err := nms.Start()
	if err != nil {
		return err
	}
	err = smf.Start(amfUrl, udmUrl)
	if err != nil {
		return err
	}
	err = amf.Start(udmUrl)
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
	cfg, err := parseFlags()
	if err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	err = cfg.Validate()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}
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
