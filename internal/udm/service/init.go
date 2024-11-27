package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/omec-project/util/path_util"
	"github.com/sirupsen/logrus"
	"github.com/yeastengine/ella/internal/udm/context"
	"github.com/yeastengine/ella/internal/udm/eventexposure"
	"github.com/yeastengine/ella/internal/udm/factory"
	"github.com/yeastengine/ella/internal/udm/httpcallback"
	"github.com/yeastengine/ella/internal/udm/logger"
	"github.com/yeastengine/ella/internal/udm/parameterprovision"
	"github.com/yeastengine/ella/internal/udm/subscriberdatamanagement"
	"github.com/yeastengine/ella/internal/udm/ueauthentication"
	"github.com/yeastengine/ella/internal/udm/uecontextmanagement"
	"github.com/yeastengine/ella/internal/udm/util"
)

type UDM struct{}

var initLog *logrus.Entry

func init() {
	initLog = logger.InitLog
}

func (udm *UDM) Initialize(c factory.Config) error {
	factory.InitConfigFactory(c)
	udm.setLogLevel()
	return nil
}

func (udm *UDM) setLogLevel() {
	level, err := logrus.ParseLevel(factory.UdmConfig.Logger.UDM.DebugLevel)
	if err != nil {
		initLog.Fatalf("UDM Log level [%s] is invalid, set to [info] level", factory.UdmConfig.Logger.UDM.DebugLevel)
	}
	initLog.Infof("UDM Log level is set to [%s] level", level)
	logger.SetLogLevel(level)
	logger.SetReportCaller(factory.UdmConfig.Logger.UDM.ReportCaller)
}

func (udm *UDM) Start() {
	config := factory.UdmConfig
	configuration := config.Configuration
	serviceName := configuration.ServiceNameList
	router := logger_util.NewGinWithLogrus(logger.GinLog)
	eventexposure.AddService(router)
	httpcallback.AddService(router)
	parameterprovision.AddService(router)
	subscriberdatamanagement.AddService(router)
	ueauthentication.AddService(router)
	uecontextmanagement.AddService(router)

	udmLogPath := path_util.Free5gcPath("omec-project/udmsslkey.log")

	self := context.UDM_Self()
	util.InitUDMContext(self)
	context.UDM_Self().InitNFService(serviceName)

	addr := fmt.Sprintf("%s:%d", self.BindingIPv4, self.SBIPort)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		udm.Terminate()
		os.Exit(0)
	}()

	server, err := http2_util.NewServer(addr, udmLogPath, router)
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

func (udm *UDM) Terminate() {
	logger.InitLog.Infof("UDM terminated")
}
