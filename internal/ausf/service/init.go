package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"

	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	ausf_context "github.com/yeastengine/ella/internal/ausf/context"
	"github.com/yeastengine/ella/internal/ausf/factory"
	"github.com/yeastengine/ella/internal/ausf/logger"
	"github.com/yeastengine/ella/internal/ausf/ueauthentication"
	"github.com/yeastengine/ella/internal/ausf/util"
)

type AUSF struct{}

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (ausf *AUSF) Initialize(c factory.Configuration) {
	factory.InitConfigFactory(c)
	ausf.setLogLevel()
}

func (ausf *AUSF) setLogLevel() {
	if level, err := logrus.ParseLevel(factory.AusfConfig.Logger.AUSF.DebugLevel); err != nil {
		initLog.Warnf("AUSF Log level [%s] is invalid, set to [info] level",
			factory.AusfConfig.Logger.AUSF.DebugLevel)
		logger.SetLogLevel(logrus.InfoLevel)
	} else {
		initLog.Infof("AUSF Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
	logger.SetReportCaller(factory.AusfConfig.Logger.AUSF.ReportCaller)
}

func (ausf *AUSF) Start() {
	router := logger_util.NewGinWithLogrus(logger.GinLog)
	ueauthentication.AddService(router)

	ausf_context.Init()
	self := ausf_context.GetSelf()

	ausfLogPath := util.AusfLogPath

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		ausf.Terminate()
		os.Exit(0)
	}()

	server, err := http2_util.NewServer(addr, ausfLogPath, router)
	if server == nil {
		initLog.Errorf("Initialize HTTP server failed: %+v", err)
		return
	}

	if err != nil {
		initLog.Warnf("Initialize HTTP server: +%v", err)
	}

	err = server.ListenAndServe()
	if err != nil {
		initLog.Fatalf("HTTP server setup failed: %+v", err)
	}
}

func (ausf *AUSF) Terminate() {
	logger.InitLog.Infof("AUSF terminated")
}
