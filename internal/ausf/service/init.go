package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/omec-project/util/http2_util"
	utilLogger "github.com/omec-project/util/logger"
	ausf_context "github.com/yeastengine/ella/internal/ausf/context"
	"github.com/yeastengine/ella/internal/ausf/factory"
	"github.com/yeastengine/ella/internal/ausf/logger"
	"github.com/yeastengine/ella/internal/ausf/ueauthentication"
)

type AUSF struct{}

func (ausf *AUSF) Initialize(c factory.Configuration) {
	factory.InitConfigFactory(c)
	ausf.setLogLevel()
}

func (ausf *AUSF) setLogLevel() {
	if level, err := zapcore.ParseLevel(factory.AusfConfig.Logger.AUSF.DebugLevel); err != nil {
		logger.InitLog.Warnf("AUSF Log level [%s] is invalid, set to [info] level",
			factory.AusfConfig.Logger.AUSF.DebugLevel)
		logger.SetLogLevel(zap.InfoLevel)
	} else {
		logger.InitLog.Infof("AUSF Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
}

func (ausf *AUSF) Start() {
	router := utilLogger.NewGinWithZap(logger.GinLog)
	ueauthentication.AddService(router)

	ausf_context.Init()
	self := ausf_context.GetSelf()

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		ausf.Terminate()
		os.Exit(0)
	}()

	server, err := http2_util.NewServer(addr, "/var/log/ausf.log", router)
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

func (ausf *AUSF) Terminate() {
	logger.InitLog.Infof("AUSF terminated")
}
