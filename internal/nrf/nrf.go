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
const port = "29510"

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
    port: %s
    registerIPv4: 127.0.0.1
    scheme: http
  serviceNameList:
  - nnrf-nfm
  - nnrf-disc
info:
  description: NRF initial local configuration
  version: 1.0.0
`, dBName, mongoDBURL, dBName, mongoDBURL, port)
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

func Start(mongoDBURL string) (string, error) {
	c, err := getContext(mongoDBURL)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return "", fmt.Errorf("failed to get context")
	}
	err = NRF.Initialize(c)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return "", fmt.Errorf("failed to initialize")
	}
	NRF.Start()
	return "http://127.0.0.1:29510", nil
}
