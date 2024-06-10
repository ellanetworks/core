package ausf

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/yeastengine/ella/internal/ausf/logger"
	"github.com/yeastengine/ella/internal/ausf/service"
)

const (
	SBI_PORT      = "29509"
	AUSF_GROUP_ID = "ausfGroup001"
)

var AUSF = &service.AUSF{}

var appLog *logrus.Entry

func init() {
	appLog = logger.AppLog
}

func getContext(nrfUrl string, webuiUrl string) (*cli.Context, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("ausfcfg", "", "AUSF configuration")
	app := cli.NewApp()
	appLog.Infoln(app.Name)
	c := cli.NewContext(app, flagSet, nil)
	ausfConfig := fmt.Sprintf(`
configuration:
  groupId: %s
  nrfUri: %s
  webuiUri: %s
  sbi:
    bindingIPv4: 0.0.0.0
    port: %s
    registerIPv4: 1.2.3.4
    scheme: http
  serviceNameList:
    - nausf-auth
info:
  description: AUSF initial local configuration
  version: 1.0.0
logger:
  AUSF:
    ReportCaller: false
    debugLevel: debug
`, AUSF_GROUP_ID, nrfUrl, webuiUrl, SBI_PORT)
	tmpFile, err := os.CreateTemp("", "ausfcfg-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = tmpFile.Write([]byte(ausfConfig))
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	err = c.Set("ausfcfg", tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		return nil, err
	}

	return c, nil
}

func Start(nrfUrl string, webuiUrl string) error {
	c, err := getContext(nrfUrl, webuiUrl)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to get context")
	}
	err = AUSF.Initialize(c)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to initialize")
	}
	go AUSF.Start()
	return nil
}
