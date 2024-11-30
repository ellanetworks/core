package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/ella/internal/nssf/context"
	"github.com/yeastengine/ella/internal/nssf/factory"
	"github.com/yeastengine/ella/internal/nssf/logger"
	"github.com/yeastengine/ella/internal/nssf/nssaiavailability"
	"github.com/yeastengine/ella/internal/nssf/nsselection"
	"github.com/yeastengine/ella/internal/nssf/util"
)

type NSSF struct{}

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (nssf *NSSF) Initialize(c factory.Configuration) {
	factory.InitConfigFactory(c)
	context.InitNssfContext()
	nssf.setLogLevel()
}

func (nssf *NSSF) setLogLevel() {
	if level, err := logrus.ParseLevel(factory.NssfConfig.Logger.NSSF.DebugLevel); err != nil {
		initLog.Warnf("NSSF Log level [%s] is invalid, set to [info] level",
			factory.NssfConfig.Logger.NSSF.DebugLevel)
		logger.SetLogLevel(logrus.InfoLevel)
	} else {
		initLog.Infof("NSSF Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
	logger.SetReportCaller(factory.NssfConfig.Logger.NSSF.ReportCaller)
}

func (nssf *NSSF) Start() {
	router := logger_util.NewGinWithLogrus(logger.GinLog)

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

	server, err := http2_util.NewServer(addr, util.NSSF_LOG_PATH, router)

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

func (nssf *NSSF) Terminate() {
	logger.InitLog.Infof("NSSF terminated")
}
