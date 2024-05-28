package amf

import (
	"flag"
	"fmt"
	"os"

	"github.com/omec-project/amf/logger"
	"github.com/omec-project/amf/service"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var AMF = &service.AMF{}

var appLog *logrus.Entry

const dBName = "sdcore_amf"

func init() {
	appLog = logger.AppLog
}

func getContext(mongoDBURL string) (*cli.Context, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("amfcfg", "", "AMF configuration")
	app := cli.NewApp()
	appLog.Infoln(app.Name)
	c := cli.NewContext(app, flagSet, nil)
	amfConfig := fmt.Sprintf(`
configuration:
  amfDBName: %s
  amfName: AMF
  debugProfilePort: 5001
  enableDBStore: false
  enableSctpLb: false
  mongodb:
    url: %s
  networkFeatureSupport5GS:
    emc: 0
    emcN3: 0
    emf: 0
    enable: true
    imsVoPS: 0
    iwkN26: 0
    mcsi: 0
    mpsi: 0
  ngapIpList:
    - 0.0.0.0
  ngappPort: 38412
  nrfUri: https://127.0.0.1:29510
  sbi:
    bindingIPv4: 0.0.0.0
    port: 29518
    registerIPv4: 0.0.0.0
    scheme: http
  sctpGrpcPort: 9000
  serviceNameList:
    - namf-comm
    - namf-evts
    - namf-mt
    - namf-loc
    - namf-oam
  supportDnnList:
    - internet
  security:
    integrityOrder:
      - NIA1
      - NIA2
    cipheringOrder:
      - NEA0
  networkName:
    full: SDCORE5G
    short: SDCORE
  t3502Value: 720
  t3512Value: 3600
  t3513:
    enable: true
    expireTime: 6s
    maxRetryTimes: 4
  t3522:
    enable: true
    expireTime: 6s
    maxRetryTimes: 4
  t3550:
    enable: true
    expireTime: 6s
    maxRetryTimes: 4
  t3560:
    enable: true
    expireTime: 6s
    maxRetryTimes: 4
  t3565:
    enable: true
    expireTime: 6s
    maxRetryTimes: 4
info:
  description: AMF initial configuration
  version: 1.0.0
`, dBName, mongoDBURL)
	tmpFile, err := os.CreateTemp("", "amfcfg-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	_, err = tmpFile.Write([]byte(amfConfig))
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to write to temp file: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	err = c.Set("amfcfg", tmpFile.Name())
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
	err = AMF.Initialize(c)
	if err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to initialize")
	}
	AMF.Start()
	return nil
}
