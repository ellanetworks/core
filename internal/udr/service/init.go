package service

import (
	"github.com/yeastengine/ella/internal/udr/factory"
	"github.com/yeastengine/ella/internal/udr/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type UDR struct{}

func (udr *UDR) Initialize(c factory.Configuration) {
	factory.InitConfigFactory(c)
	udr.setLogLevel()
}

func (udr *UDR) setLogLevel() {
	if level, err := zapcore.ParseLevel(factory.UdrConfig.Logger.UDR.DebugLevel); err != nil {
		logger.InitLog.Warnf("UDR Log level [%s] is invalid, set to [info] level",
			factory.UdrConfig.Logger.UDR.DebugLevel)
		logger.SetLogLevel(zap.InfoLevel)
	} else {
		logger.InitLog.Infof("UDR Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
}
