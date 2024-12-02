package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	udr_context "github.com/yeastengine/ella/internal/udr/context"
	"github.com/yeastengine/ella/internal/udr/datarepository"
	"github.com/yeastengine/ella/internal/udr/factory"
	"github.com/yeastengine/ella/internal/udr/logger"
	"github.com/yeastengine/ella/internal/udr/producer"
	"github.com/yeastengine/ella/internal/udr/util"
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

func (udr *UDR) Start() {
	config := factory.UdrConfig
	mongodb := config.Mongodb

	producer.ConnectMongo(mongodb.Url, mongodb.Name)

	router := logger_util.NewGinWithZap(logger.GinLog)

	datarepository.AddService(router)

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

	server, err := http2_util.NewServer(addr, "/var/log/udr.log", router)
	if server == nil {
		logger.InitLog.Errorf("Initialize HTTP server failed: %+v", err)
		return
	}

	if err != nil {
		logger.InitLog.Warnf("Initialize HTTP server: %+v", err)
	}

	err = server.ListenAndServe()
	if err != nil {
		logger.InitLog.Fatalf("HTTP server setup failed: %+v", err)
	}
}

func (udr *UDR) Terminate() {
	logger.InitLog.Infof("UDR terminated")
}
