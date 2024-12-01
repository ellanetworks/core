package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/yeastengine/ella/internal/amf"
	"github.com/yeastengine/ella/internal/ausf"
	"github.com/yeastengine/ella/internal/config"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/nssf"
	"github.com/yeastengine/ella/internal/pcf"
	"github.com/yeastengine/ella/internal/smf"
	"github.com/yeastengine/ella/internal/udm"
	"github.com/yeastengine/ella/internal/udr"
	"github.com/yeastengine/ella/internal/upf"
	"github.com/yeastengine/ella/internal/webui"
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
	ausfUrl := "http://127.0.0.1:29509"
	amfUrl := "http://127.0.0.1:29518"
	nssfUrl := "http://127.0.0.1:29531"
	pcfUrl := "http://127.0.0.1:29507"
	smfUrl := "http://127.0.0.1:29502"
	udmUrl := "http://127.0.0.1:29503"
	udrUrl := "http://127.0.0.1:29504"
	err := smf.Start(amfUrl, pcfUrl, udmUrl)
	if err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	_, err = webui.Start(cfg.DB.Url, cfg.DB.Name)
	if err != nil {
		return err
	}
	err = amf.Start(ausfUrl, nssfUrl, pcfUrl, smfUrl, udmUrl, udmUrl)
	if err != nil {
		return err
	}
	err = ausf.Start(udmUrl)
	if err != nil {
		return err
	}
	err = pcf.Start(amfUrl, udrUrl)
	if err != nil {
		return err
	}
	err = udr.Start(cfg.DB.Url, cfg.DB.Name)
	if err != nil {
		return err
	}
	err = udm.Start(udrUrl)
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
	err = db.TestConnection(cfg.DB.Url)
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}
	err = startNetwork(cfg)
	if err != nil {
		panic(err)
	}
	select {}
}
