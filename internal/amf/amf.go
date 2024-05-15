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

func getContext(amfConfigFile string) *cli.Context {
	flagSet := flag.NewFlagSet("test", flag.ContinueOnError)
	flagSet.String("amfcfg", "", "AMF configuration")
	app := cli.NewApp()
	appLog.Infoln(app.Name)
	c := cli.NewContext(app, flagSet, nil)
	c.Set("amfcfg", amfConfigFile)
	return c
}

func Start(amfConfigFile string) error {
	c := getContext(amfConfigFile)
	if err := AMF.Initialize(c); err != nil {
		logger.CfgLog.Errorf("%+v", err)
		return fmt.Errorf("failed to initialize")
	}
	AMF.Start()
	return nil
}
