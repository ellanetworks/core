package service

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/amf/context"
	"github.com/yeastengine/ella/internal/amf/factory"
	"github.com/yeastengine/ella/internal/amf/logger"
	"github.com/yeastengine/ella/internal/amf/ngap"
	ngap_message "github.com/yeastengine/ella/internal/amf/ngap/message"
	ngap_service "github.com/yeastengine/ella/internal/amf/ngap/service"
	"github.com/yeastengine/ella/internal/amf/producer/callback"
	"github.com/yeastengine/ella/internal/amf/util"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type AMF struct{}

const IMSI_PREFIX = "imsi-"

func (amf *AMF) Initialize(c factory.Configuration) {
	factory.InitConfigFactory(c)
	amf.setLogLevel()
}

func (amf *AMF) setLogLevel() {
	if level, err := zapcore.ParseLevel(factory.AmfConfig.Logger.AMF.DebugLevel); err != nil {
		logger.InitLog.Warnf("AMF Log level [%s] is invalid, set to [info] level",
			factory.AmfConfig.Logger.AMF.DebugLevel)
		logger.SetLogLevel(zap.InfoLevel)
	} else {
		logger.InitLog.Infof("AMF Log level is set to [%s] level", level)
		logger.SetLogLevel(level)
	}
}

func (amf *AMF) Start() {
	self := context.AMF_Self()
	util.InitAmfContext(self)

	ngapHandler := ngap_service.NGAPHandler{
		HandleMessage:      ngap.Dispatch,
		HandleNotification: ngap.HandleSCTPNotification,
	}
	ngap_service.Run(self.NgapIpList, self.NgapPort, ngapHandler)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		amf.Terminate()
		os.Exit(0)
	}()
}

// Used in AMF planned removal procedure
func (amf *AMF) Terminate() {
	logger.InitLog.Infof("Terminating AMF...")
	amfSelf := context.AMF_Self()

	// send AMF status indication to ran to notify ran that this AMF will be unavailable
	logger.InitLog.Infof("Send AMF Status Indication to Notify RANs due to AMF terminating")
	guamiList := context.GetServedGuamiList()
	unavailableGuamiList := ngap_message.BuildUnavailableGUAMIList(guamiList)
	amfSelf.AmfRanPool.Range(func(key, value interface{}) bool {
		ran := value.(*context.AmfRan)
		ngap_message.SendAMFStatusIndication(ran, unavailableGuamiList)
		return true
	})

	ngap_service.Stop()

	callback.SendAmfStatusChangeNotify((string)(models.StatusChange_UNAVAILABLE), guamiList)

	logger.InitLog.Infof("AMF terminated")
}
