package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/sirupsen/logrus"
	udr_context "github.com/yeastengine/ella/internal/udr/context"
	"github.com/yeastengine/ella/internal/udr/datarepository"
	"github.com/yeastengine/ella/internal/udr/factory"
	"github.com/yeastengine/ella/internal/udr/logger"
	"github.com/yeastengine/ella/internal/udr/producer"
	"github.com/yeastengine/ella/internal/udr/util"
)

type UDR struct{}

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (udr *UDR) Initialize(c factory.Configuration) {
	factory.InitConfigFactory(c)
	udr.setLogLevel()
}

func (udr *UDR) setLogLevel() {
	if level, err := logrus.ParseLevel(factory.UdrConfig.Logger.UDR.DebugLevel); err != nil {
		initLog.Warnf("UDR Log level [%s] is invalid, set to [info] level",
			factory.UdrConfig.Logger.UDR.DebugLevel)
		logger.SetLogLevel(logrus.InfoLevel)
	} else {
		initLog.Infof("UDR Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
	logger.SetReportCaller(factory.UdrConfig.Logger.UDR.ReportCaller)
}

func (udr *UDR) Start() {
	config := factory.UdrConfig
	mongodb := config.Mongodb

	producer.ConnectMongo(mongodb.Url, mongodb.Name)

	router := logger_util.NewGinWithLogrus(logger.GinLog)

	datarepository.AddService(router)

	udrLogPath := util.UdrLogPath

	self := udr_context.UDR_Self()
	util.InitUdrContext(self)

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		udr.Terminate()
		os.Exit(0)
	}()

	server, err := http2_util.NewServer(addr, udrLogPath, router)
	if server == nil {
		initLog.Errorf("Initialize HTTP server failed: %+v", err)
		return
	}

	if err != nil {
		initLog.Warnf("Initialize HTTP server: %+v", err)
	}

	err = server.ListenAndServe()
	if err != nil {
		initLog.Fatalf("HTTP server setup failed: %+v", err)
	}
}

func (udr *UDR) Terminate() {
	logger.InitLog.Infof("UDR terminated")
}
