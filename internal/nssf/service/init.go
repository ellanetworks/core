package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/nssf/context"
	"github.com/yeastengine/ella/internal/nssf/factory"
	"github.com/yeastengine/ella/internal/nssf/logger"
	"github.com/yeastengine/ella/internal/nssf/nssaiavailability"
	"github.com/yeastengine/ella/internal/nssf/nsselection"
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
	router := logger_util.NewGinWithZap(logger.GinLog)

	nssaiavailability.AddService(router)
	nsselection.AddService(router)

	self := context.NSSF_Self()
	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		nssf.Terminate()
		os.Exit(0)
	}()

	server, err := http2_util.NewServer(addr, "/var/log/nssf.log", router)

	if server == nil {
		logger.InitLog.Errorf("Initialize HTTP server failed: %+v", err)
		return
	}

	if err != nil {
		logger.InitLog.Warnf("Initialize HTTP server: +%v", err)
	}

	err = server.ListenAndServe()
	if err != nil {
		logger.InitLog.Fatalf("HTTP server setup failed: %+v", err)
	}
}

func (nssf *NSSF) Terminate() {
	logger.InitLog.Infof("NSSF terminated")
}
