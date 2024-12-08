package service

import (
	"github.com/yeastengine/ella/internal/pcf/context"
	"github.com/yeastengine/ella/internal/pcf/factory"
	"github.com/yeastengine/ella/internal/pcf/internal/notifyevent"
	"github.com/yeastengine/ella/internal/pcf/logger"
	"github.com/yeastengine/ella/internal/pcf/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type PCF struct{}

func (pcf *PCF) Initialize(c factory.Configuration) {
	factory.InitConfigFactory(c)
	pcf.setLogLevel()
}

func (pcf *PCF) setLogLevel() {
	if level, err := zapcore.ParseLevel(factory.PcfConfig.Logger.PCF.DebugLevel); err != nil {
		logger.InitLog.Warnf("PCF Log level [%s] is invalid, set to [info] level",
			factory.PcfConfig.Logger.PCF.DebugLevel)
		logger.SetLogLevel(zap.InfoLevel)
	} else {
		logger.InitLog.Infof("PCF Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
}

func (pcf *PCF) Start() {
	if err := notifyevent.RegisterNotifyDispatcher(); err != nil {
		logger.InitLog.Error("Register NotifyDispatcher Error")
	}
	self := context.PCF_Self()
	util.InitpcfContext(self)
}

func (pcf *PCF) Terminate() {
	logger.InitLog.Infof("PCF terminated")
}
