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

const dBName = "free5gc"
const authDbName = "authentication"

func getContext(dbUrl string) (*cli.Context, error) {
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
`, dBName, dbUrl, authDbName, dbUrl)
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

func Start(dbUrl string) error {
	c, err := getContext(dbUrl)
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
