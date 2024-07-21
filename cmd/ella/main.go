package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/yeastengine/ella/internal/amf"
	"github.com/yeastengine/ella/internal/ausf"
	"github.com/yeastengine/ella/internal/config"
	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/nrf"
	"github.com/yeastengine/ella/internal/nssf"
	"github.com/yeastengine/ella/internal/pcf"
	"github.com/yeastengine/ella/internal/smf"
	"github.com/yeastengine/ella/internal/udm"
	"github.com/yeastengine/ella/internal/udr"
	"github.com/yeastengine/ella/internal/upf"
	"github.com/yeastengine/ella/internal/webui"
)

const DBPath = "/var/snap/ella/common/data"

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

func startMongoDB(mongoBinariesPath string) string {
	db, err := db.StartMongoDB(mongoBinariesPath, DBPath)
	if err != nil {
		panic(err)
	}
	return db.URL
}

func setEnvironmentVariables() error {
	err := os.Setenv("POD_IP", "0.0.0.0")
	if err != nil {
		return err
	}
	err = os.Setenv("PFCP_PORT_UPF", "8806")
	if err != nil {
		return err
	}
	return nil
}

func startNetwork(cfg config.Config) error {
	dbUrl := startMongoDB(cfg.MongoDBBinariesPath)

	webuiUrl, err := webui.Start(dbUrl)
	if err != nil {
		return err
	}
	nrfUrl, err := nrf.Start(dbUrl, webuiUrl)
	if err != nil {
		return err
	}
	err = amf.Start(dbUrl, nrfUrl, webuiUrl)
	if err != nil {
		return err
	}
	err = ausf.Start(nrfUrl, webuiUrl)
	if err != nil {
		return err
	}
	err = pcf.Start(nrfUrl, webuiUrl)
	if err != nil {
		return err
	}
	err = udr.Start(dbUrl, nrfUrl, webuiUrl)
	if err != nil {
		return err
	}
	err = udm.Start(nrfUrl, webuiUrl)
	if err != nil {
		return err
	}
	err = nssf.Start(nrfUrl, webuiUrl)
	if err != nil {
		return err
	}
	err = smf.Start(dbUrl, nrfUrl, webuiUrl)
	if err != nil {
		return err
	}
	err = upf.Start()
	if err != nil {
		return err
	}

	return nil
}

func main() {
	err := setEnvironmentVariables()
	if err != nil {
		log.Fatalf("failed to set environment variables: %v", err)
	}
	cfg, err := parseFlags()
	if err != nil {
		log.Fatalf("failed to parse flags: %v", err)
	}
	err = cfg.Validate()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}
	err = startNetwork(cfg)
	if err != nil {
		panic(err)
	}
	select {}
}
