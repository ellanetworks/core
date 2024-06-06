package webui

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/yeastengine/canard/internal/webui/backend/logger"
	"github.com/yeastengine/canard/internal/webui/backend/webui_service"
)

var WEBUI = &webui_service.WEBUI{}

var appLog *logrus.Entry

func init() {
	appLog = logger.AppLog
}

const (
	dBName     = "free5gc"
	authDbName = "authentication"
	GRPC_PORT  = "9876"
)

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
logger:
  WEBUI:
    debugLevel: debug
    ReportCaller: false
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

func Start(dbUrl string) (string, error) {
	c, err := getContext(dbUrl)
	if err != nil {
		logger.ConfigLog.Errorf("%+v", err)
		return "", fmt.Errorf("failed to get context")
	}
	WEBUI.Initialize(c)
	go WEBUI.Start()
	return "localhost:" + GRPC_PORT, nil
}
