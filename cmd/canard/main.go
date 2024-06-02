package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yeastengine/canard/internal/amf"
	"github.com/yeastengine/canard/internal/config"
	"github.com/yeastengine/canard/internal/db"
	"github.com/yeastengine/canard/internal/nrf"
	"github.com/yeastengine/canard/internal/webui"
)

const DBPath = "/var/snap/canard/common/data"

type NRF struct {
	URL string
}

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

func startNRF(dbUrl string) (string, error) {
	nrfObj := NRF{}
	go func() {
		url, err := nrf.Start(dbUrl)
		if err != nil {
			panic(err)
		}
		nrfObj.URL = url
	}()

	return nrfObj.URL, nil
}

func startNetworkFunctionServices(cfg config.Config, dbUrl string, nrfUrl string) {
	go func() {
		err := webui.Start(dbUrl)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		err := amf.Start(dbUrl, nrfUrl)
		if err != nil {
			panic(err)
		}
	}()
}

func startMongoDB() string {
	db, err := db.StartMongoDB(DBPath)
	if err != nil {
		panic(err)
	}
	return db.URL
}

func main() {
	os.Setenv("MANAGED_BY_CONFIG_POD", "true")
	cfg, err := parseFlags()
	if err != nil {
		panic(err)
	}
	dbUrl := startMongoDB()
	nrfUrl, err := startNRF(dbUrl)
	if err != nil {
		panic(err)
	}
	startNetworkFunctionServices(cfg, dbUrl, nrfUrl)
	select {}
}
