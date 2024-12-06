package service

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/yeastengine/ella/internal/nssf/context"
	"github.com/yeastengine/ella/internal/nssf/factory"
	"github.com/yeastengine/ella/internal/nssf/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type NSSF struct{}

func (nssf *NSSF) Initialize(c factory.Configuration) {
	factory.InitConfigFactory(c)
	context.InitNssfContext()
	nssf.setLogLevel()
}

func (nssf *NSSF) setLogLevel() {
	if level, err := zapcore.ParseLevel(factory.NssfConfig.Logger.NSSF.DebugLevel); err != nil {
		logger.InitLog.Warnf("NSSF Log level [%s] is invalid, set to [info] level",
			factory.NssfConfig.Logger.NSSF.DebugLevel)
		logger.SetLogLevel(zap.InfoLevel)
	} else {
		logger.InitLog.Infof("NSSF Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
}

func (nssf *NSSF) Start() {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		nssf.Terminate()
		os.Exit(0)
	}()
}

func (nssf *NSSF) Terminate() {
	logger.InitLog.Infof("NSSF terminated")
}
