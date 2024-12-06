package service

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/omec-project/util/http2_util"
	logger_util "github.com/omec-project/util/logger"
	"github.com/yeastengine/ella/internal/udm/context"
	"github.com/yeastengine/ella/internal/udm/eventexposure"
	"github.com/yeastengine/ella/internal/udm/factory"
	"github.com/yeastengine/ella/internal/udm/httpcallback"
	"github.com/yeastengine/ella/internal/udm/logger"
	"github.com/yeastengine/ella/internal/udm/subscriberdatamanagement"
	"github.com/yeastengine/ella/internal/udm/ueauthentication"
	"github.com/yeastengine/ella/internal/udm/uecontextmanagement"
	"github.com/yeastengine/ella/internal/udm/util"
	"go.uber.org/zap/zapcore"
)

type UDM struct{}

func (udm *UDM) Initialize(c factory.Configuration) error {
	factory.InitConfigFactory(c)
	udm.setLogLevel()
	return nil
}

func (udm *UDM) setLogLevel() {
	level, err := zapcore.ParseLevel(factory.UdmConfig.Logger.UDM.DebugLevel)
	if err != nil {
		logger.InitLog.Fatalf("UDM Log level [%s] is invalid, set to [info] level", factory.UdmConfig.Logger.UDM.DebugLevel)
	}
	logger.InitLog.Infof("UDM Log level is set to [%s] level", level)
	logger.SetLogLevel(level)
}

func (udm *UDM) Start() {
	config := factory.UdmConfig
	serviceName := config.ServiceNameList
	router := logger_util.NewGinWithZap(logger.GinLog)
	eventexposure.AddService(router)
	httpcallback.AddService(router)
	subscriberdatamanagement.AddService(router)
	ueauthentication.AddService(router)
	uecontextmanagement.AddService(router)

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

	server, err := http2_util.NewServer(addr, "/var/log/udm.log", router)
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

func (udm *UDM) Terminate() {
	logger.InitLog.Infof("UDM terminated")
}
