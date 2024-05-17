package webui

import (
	"flag"
	"fmt"

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

func getContext(webuiConfigFile string) (*cli.Context, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("webuicfg", "", "WEBUI configuration")
	app := cli.NewApp()
	appLog.Infoln(app.Name)
	c := cli.NewContext(app, flagSet, nil)
	err := c.Set("webuicfg", webuiConfigFile)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func Start(webuiConfigFile string) error {
	c, err := getContext(webuiConfigFile)
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
