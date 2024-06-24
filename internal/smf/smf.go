package smf

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/service"
)

var SMF = &service.SMF{}

var appLog *logrus.Entry

const (
	dBName   = "smf"
	SBI_PORT = "29502"
)

func init() {
	appLog = logger.AppLog
}

func getContext(mongoDBURL string, nrfURL string, webuiURL string) (*cli.Context, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("smfcfg", "", "SMF configuration")
	flagSet.String("uerouting", "", "UE routing information")
	app := cli.NewApp()
	app.Name = "smf"
	app.Flags = SMF.GetCliCmd()
	appLog.Infoln(app.Name)
	c := cli.NewContext(app, flagSet, nil)
	smfConfig := fmt.Sprintf(`
info:
  version: 1.0.0
  description: SMF initial local configuration

configuration:
  smfDBName: %s
  webuiUri: %s
  enableDBStore: false
  enableUPFAdapter: false
  debugProfilePort: 5001
  mongodb:
    url: %s
  kafkaInfo:
    enableKafka: false
  smfName: SMF
  sbi:
    scheme: http
    registerIPv4: 0.0.0.0
    bindingIPv4: 0.0.0.0
    port: %s
  serviceNameList:
    - nsmf-pdusession
    - nsmf-event-exposure
    - nsmf-oam
  pfcp:
    addr: 0.0.0.0
  nrfUri: %s
logger:
  SMF:
    debugLevel: debug
    ReportCaller: false
`, dBName, webuiURL, mongoDBURL, SBI_PORT, nrfURL)
	tmpFile, err := os.CreateTemp("", "smfcfg-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = tmpFile.Write([]byte(smfConfig))
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	err = c.Set("smfcfg", tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		return nil, err
	}

	ueroutingcfg := fmt.Sprintf(`
info:
  description: Routing information for UE
  version: 1.0.0
`)
	tmpFile, err = os.CreateTemp("", "uerouting-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = tmpFile.Write([]byte(ueroutingcfg))
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	err = c.Set("uerouting", tmpFile.Name())
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
	err = SMF.Initialize(c)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to initialize")
	}
	go SMF.Start()
	return nil
}
