package service

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	ngap_message "github.com/ellanetworks/core/internal/amf/ngap/message"
	ngap_service "github.com/ellanetworks/core/internal/amf/ngap/service"
	"github.com/ellanetworks/core/internal/amf/producer/callback"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/openapi/models"
)

type AMF struct{}

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
	logger.AmfLog.Infof("Terminating AMF...")
	amfSelf := context.AMF_Self()

	// send AMF status indication to ran to notify ran that this AMF will be unavailable
	logger.AmfLog.Infof("Send AMF Status Indication to Notify RANs due to AMF terminating")
	guamiList := context.GetServedGuamiList()
	unavailableGuamiList := ngap_message.BuildUnavailableGUAMIList(guamiList)
	amfSelf.AmfRanPool.Range(func(key, value interface{}) bool {
		ran := value.(*context.AmfRan)
		ngap_message.SendAMFStatusIndication(ran, unavailableGuamiList)
		return true
	})

	ngap_service.Stop()

	callback.SendAmfStatusChangeNotify((string)(models.StatusChange_UNAVAILABLE), guamiList)

	logger.AmfLog.Infof("AMF terminated")
}
