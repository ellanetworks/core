package pcf

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/yeastengine/canard/internal/pcf/logger"
	"github.com/yeastengine/canard/internal/pcf/service"
)

var PCF = &service.PCF{}

var appLog *logrus.Entry

const SBI_PORT = "29507"

func init() {
	appLog = logger.AppLog
}

func getContext(nrfURL string, webuiURL string) (*cli.Context, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("pcfcfg", "", "PCF configuration")
	app := cli.NewApp()
	appLog.Infoln(app.Name)
	c := cli.NewContext(app, flagSet, nil)
	pcfConfig := fmt.Sprintf(`
configuration:
  defaultBdtRefId: BdtPolicyId-
  nrfUri: %s
  pcfName: PCF
  webuiUri: %s
  sbi:
    bindingIPv4: 0.0.0.0
    port: %s
    registerIPv4: 0.0.0.0
    scheme: http
  serviceList:
  - serviceName: npcf-am-policy-control
  - serviceName: npcf-smpolicycontrol
    suppFeat: 3fff
  - serviceName: npcf-bdtpolicycontrol
  - serviceName: npcf-policyauthorization
    suppFeat: 3
  - serviceName: npcf-eventexposure
  - serviceName: npcf-ue-policy-control
info:
  description: PCF initial local configuration
  version: 1.0.0
logger:
  PCF:
    ReportCaller: false
    debugLevel: info
`, nrfURL, webuiURL, SBI_PORT)
	tmpFile, err := os.CreateTemp("", "pcfcfg-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = tmpFile.Write([]byte(pcfConfig))
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	err = c.Set("pcfcfg", tmpFile.Name())
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
	err = PCF.Initialize(c)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to initialize")
	}
	go PCF.Start()
	return nil
}
