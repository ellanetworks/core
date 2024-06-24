package udm

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/yeastengine/ella/internal/udm/logger"
	"github.com/yeastengine/ella/internal/udm/service"
)

var UDM = &service.UDM{}

var appLog *logrus.Entry

const (
	UDM_HNP_PRIVATE_KEY = "c09c17bddf23357f614f492075b970d825767718114f59554ce2f345cf8c4b6a"
	SBI_PORT            = "29503"
)

func init() {
	appLog = logger.AppLog
}

func getContext(nrfURL string, webuiURL string) (*cli.Context, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("udmcfg", "", "UDM configuration")
	app := cli.NewApp()
	appLog.Infoln(app.Name)
	c := cli.NewContext(app, flagSet, nil)
	udmConfig := fmt.Sprintf(`
configuration:
  keys:
    udmProfileAHNPrivateKey: %s
  nrfUri: %s
  webuiUri: %s
  sbi:
    bindingIPv4: 0.0.0.0
    port: %s
    registerIPv4: 0.0.0.0:29503
  serviceNameList:
  - nudm-sdm
  - nudm-uecm
  - nudm-ueau
  - nudm-ee
  - nudm-pp
info:
  description: UDM initial local configuration
  version: 1.0.0
logger:
  UDM:
    ReportCaller: false
    debugLevel: debug
`, UDM_HNP_PRIVATE_KEY, nrfURL, webuiURL, SBI_PORT)
	tmpFile, err := os.CreateTemp("", "udmcfg-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = tmpFile.Write([]byte(udmConfig))
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	err = c.Set("udmcfg", tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		return nil, err
	}

	return c, nil
}

func Start(nrfURL string, webuiURL string) error {
	c, err := getContext(nrfURL, webuiURL)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to get context")
	}
	err = UDM.Initialize(c)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to initialize")
	}
	go UDM.Start()
	return nil
}
