package service

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

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

var (
	KeepAliveTimer      *time.Timer
	KeepAliveTimerMutex sync.Mutex
)

func init() {
	initLog = logger.InitLog
}

func (udr *UDR) Initialize(c factory.Config) {
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
	// get config file info
	config := factory.UdrConfig
	mongodb := config.Configuration.Mongodb
	initLog.Infof("UDR Config Info: Version[%s] Description[%s]", config.Info.Version, config.Info.Description)

	// Connect to MongoDB
	producer.ConnectMongo(mongodb.Url, mongodb.Name, mongodb.AuthUrl, mongodb.AuthKeysDbName)
	initLog.Infoln("Server started")

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

	go udr.configUpdateDb()

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

func (udr *UDR) configUpdateDb() {
	for msg := range factory.ConfigUpdateDbTrigger {
		initLog.Infof("Config update DB trigger")
		err := producer.AddEntrySmPolicyTable(
			msg.SmPolicyTable.Imsi,
			msg.SmPolicyTable.Dnn,
			msg.SmPolicyTable.Snssai)
		if err == nil {
			initLog.Infof("added entry to sm policy table success")
		} else {
			initLog.Errorf("entry add failed %+v", err)
		}
	}
}
