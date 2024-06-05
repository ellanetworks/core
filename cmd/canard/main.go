package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yeastengine/canard/internal/amf"
	"github.com/yeastengine/canard/internal/ausf"
	"github.com/yeastengine/canard/internal/config"
	"github.com/yeastengine/canard/internal/db"
	"github.com/yeastengine/canard/internal/nrf"
	"github.com/yeastengine/canard/internal/nssf"
	"github.com/yeastengine/canard/internal/pcf"
	"github.com/yeastengine/canard/internal/udm"
	"github.com/yeastengine/canard/internal/udr"
	"github.com/yeastengine/canard/internal/webui"
)

const DBPath = "/var/snap/canard/common/data"

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

func startNRF(dbUrl string, webuiUrl string) (string, error) {
	url, err := nrf.Start(dbUrl, webuiUrl)
	if err != nil {
		return "", fmt.Errorf("failed to start NRF: %w", err)
	}
	return url, nil
}

func startWebui(dbUrl string) (string, error) {
	url, err := webui.Start(dbUrl)
	if err != nil {
		return "", fmt.Errorf("failed to start WebUI: %w", err)
	}
	return url, nil
}

func startAMF(dbUrl string, nrfUrl string, webuiUrl string) error {
	err := amf.Start(dbUrl, nrfUrl, webuiUrl)
	if err != nil {
		return fmt.Errorf("failed to start AMF: %w", err)
	}
	return nil
}

func startAUSF(nrfUrl string, webuiUrl string) error {
	err := ausf.Start(nrfUrl, webuiUrl)
	if err != nil {
		return fmt.Errorf("failed to start AUSF: %w", err)
	}
	return nil
}

func startPCF(nrfUrl string, webuiUrl string) error {
	err := pcf.Start(nrfUrl, webuiUrl)
	if err != nil {
		return fmt.Errorf("failed to start PCF: %w", err)
	}
	return nil
}

func startUDR(dbUrl string, nrfUrl string, webuiUrl string) error {
	err := udr.Start(dbUrl, nrfUrl, webuiUrl)
	if err != nil {
		return fmt.Errorf("failed to start UDR: %w", err)
	}
	return nil
}

func startUDM(nrfUrl string, webuiUrl string) error {
	err := udm.Start(nrfUrl, webuiUrl)
	if err != nil {
		return fmt.Errorf("failed to start UDM: %w", err)
	}
	return nil
}

func startNSSF(dbUrl string, webuiUrl string) error {
	err := nssf.Start(dbUrl, webuiUrl)
	if err != nil {
		return fmt.Errorf("failed to start NSSF: %w", err)
	}
	return nil
}

func startMongoDB() string {
	db, err := db.StartMongoDB(DBPath)
	if err != nil {
		panic(err)
	}
	return db.URL
}

func main() {
	err := os.Setenv("MANAGED_BY_CONFIG_POD", "true")
	if err != nil {
		panic(err)
	}
	err = os.Setenv("CONFIGPOD_DEPLOYMENT", "true")
	if err != nil {
		panic(err)
	}
	err = os.Setenv("GRPC_VERBOSITY", "debug")
	if err != nil {
		panic(err)
	}
	err = os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "info")
	if err != nil {
		panic(err)
	}
	_, err = parseFlags()
	if err != nil {
		panic(err)
	}
	dbUrl := startMongoDB()
	webuiUrl, err := startWebui(dbUrl)
	if err != nil {
		panic("Failed to start WebUI")
	}
	if webuiUrl == "" {
		panic("Failed to get WebUI URL")
	}
	nrfUrl, err := startNRF(dbUrl, webuiUrl)
	if err != nil {
		panic("Failed to start NRF")
	}
	// err = startAMF(dbUrl, nrfUrl, webuiUrl)
	// if err != nil {
	// 	panic("Failed to start AMF")
	// }
	// err = startAUSF(nrfUrl, webuiUrl)
	// if err != nil {
	// 	panic("Failed to start AUSF")
	// }
	err = startPCF(nrfUrl, webuiUrl)
	if err != nil {
		panic("Failed to start PCF")
	}
	// err = startUDR(dbUrl, nrfUrl, webuiUrl)
	// if err != nil {
	// 	panic("Failed to start UDR")
	// }
	// err = startUDM(nrfUrl, webuiUrl)
	// if err != nil {
	// 	panic("Failed to start UDM")
	// }
	err = startNSSF(nrfUrl, webuiUrl)
	if err != nil {
		panic("Failed to start NSSF")
	}
	select {}
}
