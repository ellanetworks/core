package nssf

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/yeastengine/ella/internal/nssf/logger"
	"github.com/yeastengine/ella/internal/nssf/service"
)

var NSSF = &service.NSSF{}

var appLog *logrus.Entry

const (
	SD       = "010203"
	SST      = "1"
	SBI_PORT = "29531"
)

func init() {
	appLog = logger.AppLog
}

func getContext(nrfURL string, webuiURL string) (*cli.Context, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("nssfcfg", "", "NSSF configuration")
	app := cli.NewApp()
	appLog.Infoln(app.Name)
	c := cli.NewContext(app, flagSet, nil)
	nssfConfig := fmt.Sprintf(`
configuration:
  nrfUri: %s
  webuiUri: %s
  nsiList:
  - nsiInformationList:
    - nrfId: %s/nnrf-nfm/v1/nf-instances
      nsiId: 22
    snssai:
      sd: "%s"
      sst: %s
  nssfName: NSSF
  sbi:
    bindingIPv4: 0.0.0.0
    port: %s
    registerIPv4: 0.0.0.0:29531
  serviceNameList:
  - nnssf-nsselection
  - nnssf-nssaiavailability
info:
  description: NSSF initial local configuration
  version: 1.0.0
logger:
  NSSF:
    ReportCaller: false
    debugLevel: debug
`, nrfURL, webuiURL, nrfURL, SD, SST, SBI_PORT)
	tmpFile, err := os.CreateTemp("", "nssfcfg-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = tmpFile.Write([]byte(nssfConfig))
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	err = c.Set("nssfcfg", tmpFile.Name())
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
	err = NSSF.Initialize(c)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to initialize")
	}
	go NSSF.Start()
	return nil
}
