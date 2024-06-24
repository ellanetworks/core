package nrf

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/yeastengine/ella/internal/nrf/logger"
	"github.com/yeastengine/ella/internal/nrf/service"
)

var NRF = &service.NRF{}

var appLog *logrus.Entry

const (
	dBName = "nrf"
	port   = "29510"
)

func init() {
	appLog = logger.AppLog
	appLog.Infoln("NRF")
}

func getContext(mongoDBURL string, webuiUrl string) (*cli.Context, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("nrfcfg", "", "NRF configuration")
	app := cli.NewApp()
	c := cli.NewContext(app, flagSet, nil)
	nrfConfig := fmt.Sprintf(`
configuration:
  MongoDBName: %s
  MongoDBUrl: %s
  mongodb:
    name: %s
    url: %s
  webuiUri: %s
  nfKeepAliveTime: 60
  nfProfileExpiryEnable: true
  sbi:
    bindingIPv4: 0.0.0.0
    port: %s
    registerIPv4: 127.0.0.1
  serviceNameList:
  - nnrf-nfm
  - nnrf-disc
info:
  description: NRF initial local configuration
  version: 1.0.0
logger:
  NRF:
    ReportCaller: false
    debugLevel: debug
`, dBName, mongoDBURL, dBName, mongoDBURL, webuiUrl, port)
	tmpFile, err := os.CreateTemp("", "nrfcfg-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Print nrf config file
	fmt.Println(nrfConfig)

	_, err = tmpFile.Write([]byte(nrfConfig))
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	err = c.Set("nrfcfg", tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		return nil, err
	}

	return c, nil
}

func Start(mongoDBURL string, webuiUrl string) (string, error) {
	c, err := getContext(mongoDBURL, webuiUrl)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return "", fmt.Errorf("failed to get context")
	}
	err = NRF.Initialize(c)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return "", fmt.Errorf("failed to initialize")
	}
	go NRF.Start()
	return "http://127.0.0.1:29510", nil
}
