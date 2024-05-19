package main

import (
	"flag"
	"fmt"

	"github.com/yeastengine/moose/internal/amf"
	"github.com/yeastengine/moose/internal/config"
	"github.com/yeastengine/moose/internal/webui"
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

func startNetworkFunctionServices(cfg config.Config) {
	go func() {
		err := webui.Start(cfg.Database.Name, cfg.Database.Url, cfg.Database.AuthKeysDbName, cfg.Database.AuthUrl)
		if err != nil {
			panic(err)
		}
	}()

	go func() {
		err := amf.Start(cfg.Database.Url)
		if err != nil {
			panic(err)
		}
	}()
}

func main() {
	cfg, err := parseFlags()
	if err != nil {
		panic(err)
	}
	startNetworkFunctionServices(cfg)
	select {}
}
