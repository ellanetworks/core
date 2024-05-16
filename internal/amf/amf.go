package amf

import (
	"flag"
	"fmt"

	"github.com/omec-project/amf/logger"
	"github.com/omec-project/amf/service"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var AMF = &service.AMF{}

var appLog *logrus.Entry

func init() {
	appLog = logger.AppLog
}

func getContext(amfConfigFile string) (*cli.Context, error) {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("amfcfg", "", "AMF configuration")
	app := cli.NewApp()
	appLog.Infoln(app.Name)
	c := cli.NewContext(app, flagSet, nil)
	err := c.Set("amfcfg", amfConfigFile)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func Start(amfConfigFile string) error {
	c, err := getContext(amfConfigFile)
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
