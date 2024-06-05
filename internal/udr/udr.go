package udr

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/yeastengine/canard/internal/udr/logger"
	"github.com/yeastengine/canard/internal/udr/service"
)

var UDR = &service.UDR{}

var appLog *logrus.Entry

const (
	COMMON_DB_NAME = "free5gc"
	AUTH_DB_NAME   = "authentication"
	SBI_PORT       = "29504"
)

func init() {
	appLog = logger.AppLog
}

func getContext(mongoDBURL string, nrfURL string, webuiURL string) (*cli.Context, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("udrcfg", "", "UDR configuration")
	app := cli.NewApp()
	appLog.Infoln(app.Name)
	c := cli.NewContext(app, flagSet, nil)
	udrConfig := fmt.Sprintf(`
info:
  version: 1.0.0
  description: UDR initial local configuration (https://github.com/free5gc/free5gc/blob/main/config/udrcfg.yaml)

configuration:
  sbi:
    scheme: http
    registerIPv4: 0.0.0.0
    bindingIPv4: 0.0.0.0
    port: %s
  mongodb:
    name: %s
    url: %s
    authKeysDbName: %s
    authUrl: %s
  nrfUri: %s
  webuiUri: %s
logger:
  UDR:
    debugLevel: info
    ReportCaller: false
`, SBI_PORT, COMMON_DB_NAME, mongoDBURL, AUTH_DB_NAME, mongoDBURL, nrfURL, webuiURL)
	tmpFile, err := os.CreateTemp("", "udrcfg-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = tmpFile.Write([]byte(udrConfig))
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	err = c.Set("udrcfg", tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		return nil, err
	}

	return c, nil
}

func Start(mongoDBURL string, nrfURL string, webuiURL string) error {
	c, err := getContext(mongoDBURL, nrfURL, webuiURL)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to get context")
	}
	err = UDR.Initialize(c)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to initialize")
	}
	go UDR.Start()
	return nil
}
