package nrf

import (
	"flag"
	"fmt"
	"os"

	"github.com/omec-project/nrf/logger"
	"github.com/omec-project/nrf/service"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var NRF = &service.NRF{}

var appLog *logrus.Entry

const dBName = "free5gc"

func init() {
	appLog = logger.AppLog
	appLog.Infoln("NRF")
}

func getContext(mongoDBURL string) (*cli.Context, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("nrfcfg", "", "NRF configuration")
	app := cli.NewApp()
	c := cli.NewContext(app, flagSet, nil)
	nrfConfig := fmt.Sprintf(`
configuration:
  MongoDBName: %s
  MongoDBUrl: %s
  mongoDBStreamEnable: true
  mongodb:
    name: %s
    url: %s
  nfKeepAliveTime: 60
  nfProfileExpiryEnable: true
  sbi:
    bindingIPv4: 0.0.0.0
    port: 29510
    registerIPv4: 127.0.0.1
    scheme: https
  serviceNameList:
  - nnrf-nfm
  - nnrf-disc
info:
  description: NRF initial local configuration
  version: 1.0.0
logger:
  AMF:
    ReportCaller: false
    debugLevel: info
  AUSF:
    ReportCaller: false
    debugLevel: info
  Aper:
    ReportCaller: false
    debugLevel: info
  CommonConsumerTest:
    ReportCaller: false
    debugLevel: info
  FSM:
    ReportCaller: false
    debugLevel: info
  MongoDBLibrary:
    ReportCaller: false
    debugLevel: info
  N3IWF:
    ReportCaller: false
    debugLevel: info
  NAS:
    ReportCaller: false
    debugLevel: info
  NGAP:
    ReportCaller: false
    debugLevel: info
  NRF:
    ReportCaller: false
    debugLevel: info
  NamfComm:
    ReportCaller: false
    debugLevel: info
  NamfEventExposure:
    ReportCaller: false
    debugLevel: info
  NsmfPDUSession:
    ReportCaller: false
    debugLevel: info
  NudrDataRepository:
    ReportCaller: false
    debugLevel: info
  OpenApi:
    ReportCaller: false
    debugLevel: info
  PCF:
    ReportCaller: false
    debugLevel: info
  PFCP:
    ReportCaller: false
    debugLevel: info
  PathUtil:
    ReportCaller: false
    debugLevel: info
  SMF:
    ReportCaller: false
    debugLevel: info
  UDM:
    ReportCaller: false
    debugLevel: info
  UDR:
    ReportCaller: false
    debugLevel: info
  WEBUI:
    ReportCaller: false
    debugLevel: info
`, dBName, mongoDBURL, dBName, mongoDBURL)
	tmpFile, err := os.CreateTemp("", "nrfcfg-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

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

func Start(mongoDBURL string) error {
	c, err := getContext(mongoDBURL)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to get context")
	}
	err = NRF.Initialize(c)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to initialize")
	}
	NRF.Start()
	return nil
}
