package service

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/nrf/accesstoken"
	nrf_context "github.com/yeastengine/ella/internal/nrf/context"
	"github.com/yeastengine/ella/internal/nrf/dbadapter"
	"github.com/yeastengine/ella/internal/nrf/discovery"
	"github.com/yeastengine/ella/internal/nrf/factory"
	"github.com/yeastengine/ella/internal/nrf/logger"
	"github.com/yeastengine/ella/internal/nrf/management"
	"github.com/yeastengine/ella/internal/nrf/util"
)

type NRF struct{}

type Config struct {
	nrfcfg string
}

var config Config

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (nrf *NRF) Initialize(c *cli.Context) error {
	config = Config{
		nrfcfg: c.String("nrfcfg"),
	}
	if err := factory.InitConfigFactory(config.nrfcfg); err != nil {
		return err
	}
	nrf.setLogLevel()
	return nil
}

func (nrf *NRF) setLogLevel() {
	level, err := logrus.ParseLevel(factory.NrfConfig.Logger.NRF.DebugLevel)
	if err != nil {
		initLog.Warnf("NRF Log level [%s] is invalid, set to [info] level",
			factory.NrfConfig.Logger.NRF.DebugLevel)
		logger.SetLogLevel(logrus.InfoLevel)
	} else {
		initLog.Infof("NRF Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
	logger.SetReportCaller(factory.NrfConfig.Logger.NRF.ReportCaller)
}

func (nrf *NRF) Start() {
	initLog.Infoln("Server started")
	dbadapter.ConnectToDBClient(factory.NrfConfig.Configuration.MongoDBName, factory.NrfConfig.Configuration.MongoDBUrl,
		false, factory.NrfConfig.Configuration.NfProfileExpiryEnable)

	router := logger_util.NewGinWithLogrus(logger.GinLog)

	accesstoken.AddService(router)
	discovery.AddService(router)
	management.AddService(router)

	nrf_context.InitNrfContext()

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		// Waiting for other NFs to deregister
		time.Sleep(2 * time.Second)
		nrf.Terminate()
		os.Exit(0)
	}()

	bindAddr := factory.NrfConfig.GetSbiBindingAddr()
	initLog.Infof("Binding addr: [%s]", bindAddr)
	server, err := http2_util.NewServer(bindAddr, util.NrfLogPath, router)

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

func (nrf *NRF) Terminate() {
	logger.InitLog.Infof("Terminating NRF...")

	logger.InitLog.Infof("NRF terminated")
}
