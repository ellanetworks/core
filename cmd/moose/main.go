package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/yeastengine/moose/internal/config"
	"github.com/yeastengine/moose/internal/db"
	"github.com/yeastengine/moose/internal/nrf"
)

const DBPath = "/var/snap/moose/common/data"

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

func startNetworkFunctionServices(cfg config.Config, dbUrl string) {
	// go func() {
	// 	err := webui.Start(dbUrl)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// }()

	// sleep for 10 seconds to allow webui to start
	time.Sleep(10 * time.Second)

	go func() {
		err := nrf.Start(dbUrl)
		if err != nil {
			panic(err)
		}
	}()

	// go func() {
	// 	err := amf.Start(dbUrl)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// }()
}

func startMongoDB() string {
	db, err := db.StartMongoDB(DBPath)
	if err != nil {
		panic(err)
	}
	return db.URL
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		panic(err)
	}
	dbUrl := startMongoDB()
	startNetworkFunctionServices(cfg, dbUrl)
	select {}
}
