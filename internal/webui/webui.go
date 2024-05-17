package webui

import (
	"flag"
	"fmt"
	"os"

	"github.com/omec-project/webconsole/backend/logger"
	"github.com/omec-project/webconsole/backend/webui_service"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var WEBUI = &webui_service.WEBUI{}

var appLog *logrus.Entry

func init() {
	appLog = logger.AppLog
}

func getContext(mongoDBName string, mongoDBUrl string, mongoDBKeysDBName string, mongoDBKeysUrl string) (*cli.Context, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("webuicfg", "", "Webui configuration")
	app := cli.NewApp()
	appLog.Infoln(app.Name)
	c := cli.NewContext(app, flagSet, nil)
	webuiConfig := fmt.Sprintf(`
configuration:
  managedByConfigPod:
    enabled: true
    syncUrl: ""
  mongodb:
    name: %s
    url: %s
    authKeysDbName: %s
    authUrl: %s
  spec-compliant-sdf: false
info:
  description: WebUI initial local configuration
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
`, mongoDBName, mongoDBUrl, mongoDBKeysDBName, mongoDBKeysUrl)
	tmpFile, err := os.CreateTemp("", "webuicfg-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = tmpFile.Write([]byte(webuiConfig))
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	err = c.Set("webuicfg", tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		return nil, err
	}

	return c, nil
}

func Start(dbName string, dbUrl string, dbAuthKeysName string, dbAuthUrl string) error {
	c, err := getContext(dbName, dbUrl, dbAuthKeysName, dbAuthUrl)
	if err != nil {
		logger.ConfigLog.Errorf("%+v", err)
		return fmt.Errorf("failed to get context")
	}
	WEBUI.Initialize(c)
	if err != nil {
		logger.ConfigLog.Errorf("%+v", err)
		return fmt.Errorf("failed to initialize")
	}
	WEBUI.Start()
	return nil
}
