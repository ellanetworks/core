package factory

import (
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/config5g/proto/client"
	"github.com/yeastengine/ella/internal/nrf/logger"
)

var NrfConfig Config

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func InitConfigFactory(c Config) error {
	NrfConfig = c
	initLog.Infof("DefaultPlmnId Mnc %v , Mcc %v \n", NrfConfig.Configuration.DefaultPlmnId.Mnc, NrfConfig.Configuration.DefaultPlmnId.Mcc)
	commChannel := client.ConfigWatcher(NrfConfig.Configuration.WebuiUri, "nrf")
	go NrfConfig.updateConfig(commChannel)
	return nil
}
