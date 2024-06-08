package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	mongoDBLibLogger "github.com/omec-project/util/logger"
	"github.com/omec-project/util/path_util"
	pathUtilLogger "github.com/omec-project/util/path_util/logger"
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

type (
	// Config information.
	Config struct {
		nrfcfg string
	}
)

var config Config

var nrfCLi = []cli.Flag{
	cli.StringFlag{
		Name:  "free5gccfg",
		Usage: "common config file",
	},
	cli.StringFlag{
		Name:  "nrfcfg",
		Usage: "config file",
	},
}

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (*NRF) GetCliCmd() (flags []cli.Flag) {
	return nrfCLi
}

func (nrf *NRF) Initialize(c *cli.Context) error {
	config = Config{
		nrfcfg: c.String("nrfcfg"),
	}

	if config.nrfcfg != "" {
		if err := factory.InitConfigFactory(config.nrfcfg); err != nil {
			return err
		}
	} else {
		DefaultNrfConfigPath := path_util.Free5gcPath("free5gc/config/nrfcfg.yaml")
		if err := factory.InitConfigFactory(DefaultNrfConfigPath); err != nil {
			return err
		}
	}

	nrf.setLogLevel()

	return nil
}

func (nrf *NRF) setLogLevel() {
	if factory.NrfConfig.Logger == nil {
		initLog.Warnln("NRF config without log level setting!!!")
		return
	}

	if factory.NrfConfig.Logger.NRF != nil {
		if factory.NrfConfig.Logger.NRF.DebugLevel != "" {
			level, err := logrus.ParseLevel(factory.NrfConfig.Logger.NRF.DebugLevel)
			if err != nil {
				initLog.Warnf("NRF Log level [%s] is invalid, set to [info] level",
					factory.NrfConfig.Logger.NRF.DebugLevel)
				logger.SetLogLevel(logrus.InfoLevel)
			} else {
				initLog.Infof("NRF Log level is set to [%s] level", level)
				logger.SetLogLevel(level)
			}
		} else {
			initLog.Infoln("NRF Log level not set. Default set to [info] level")
			logger.SetLogLevel(logrus.InfoLevel)
		}
		logger.SetReportCaller(factory.NrfConfig.Logger.NRF.ReportCaller)
	}

	if factory.NrfConfig.Logger.PathUtil != nil {
		if factory.NrfConfig.Logger.PathUtil.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.NrfConfig.Logger.PathUtil.DebugLevel); err != nil {
				pathUtilLogger.PathLog.Warnf("PathUtil Log level [%s] is invalid, set to [info] level",
					factory.NrfConfig.Logger.PathUtil.DebugLevel)
				pathUtilLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				pathUtilLogger.SetLogLevel(level)
			}
		} else {
			pathUtilLogger.PathLog.Warnln("PathUtil Log level not set. Default set to [info] level")
			pathUtilLogger.SetLogLevel(logrus.InfoLevel)
		}
		pathUtilLogger.SetReportCaller(factory.NrfConfig.Logger.PathUtil.ReportCaller)
	}

	/*if factory.NrfConfig.Logger.OpenApi != nil {
		if factory.NrfConfig.Logger.OpenApi.DebugLevel != "" {
			if _, err := logrus.ParseLevel(factory.NrfConfig.Logger.OpenApi.DebugLevel); err != nil {
				logger.OpenapiLog.Warnf("OpenAPI Log level [%s] is invalid, set to [info] level",
					factory.NrfConfig.Logger.OpenApi.DebugLevel)
			}
		} else {
			logger.OpenapiLog.Warnln("OpenAPI Log level not set. Default set to [info] level")
		}
		logger.SetReportCaller(factory.NrfConfig.Logger.OpenApi.ReportCaller)
	}*/

	if factory.NrfConfig.Logger.MongoDBLibrary != nil {
		if factory.NrfConfig.Logger.MongoDBLibrary.DebugLevel != "" {
			if level, err := logrus.ParseLevel(factory.NrfConfig.Logger.MongoDBLibrary.DebugLevel); err != nil {
				mongoDBLibLogger.AppLog.Warnf("MongoDBLibrary Log level [%s] is invalid, set to [info] level",
					factory.NrfConfig.Logger.MongoDBLibrary.DebugLevel)
				mongoDBLibLogger.SetLogLevel(logrus.InfoLevel)
			} else {
				mongoDBLibLogger.SetLogLevel(level)
			}
		} else {
			mongoDBLibLogger.AppLog.Warnln("MongoDBLibrary Log level not set. Default set to [info] level")
			mongoDBLibLogger.SetLogLevel(logrus.InfoLevel)
		}
		mongoDBLibLogger.SetReportCaller(factory.NrfConfig.Logger.MongoDBLibrary.ReportCaller)
	}
}

func (nrf *NRF) FilterCli(c *cli.Context) (args []string) {
	for _, flag := range nrf.GetCliCmd() {
		name := flag.GetName()
		value := fmt.Sprint(c.Generic(name))
		if value == "" {
			continue
		}

		args = append(args, "--"+name, value)
	}
	return args
}

func (nrf *NRF) Start() {
	initLog.Infoln("Server started")
	dbadapter.ConnectToDBClient(factory.NrfConfig.Configuration.MongoDBName, factory.NrfConfig.Configuration.MongoDBUrl,
		factory.NrfConfig.Configuration.MongoDBStreamEnable, factory.NrfConfig.Configuration.NfProfileExpiryEnable)

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
		panic("HTTP server setup failed")
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
