package amf

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/amf/ngap/service"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas/security"
	"github.com/omec-project/openapi/models"
)

const (
	NGAPP_PORT = 38412
)

func Start(dbInstance *db.Database) error {
	self := context.AMF_Self()
	self.Name = "AMF"
	self.NgapIpList = []string{"0.0.0.0"}
	self.NgapPort = NGAPP_PORT
	self.SctpGrpcPort = 9000
	self.NetworkFeatureSupport5GS = &context.NetworkFeatureSupport5GS{
		Emc:     0,
		EmcN3:   0,
		Emf:     0,
		Enable:  true,
		ImsVoPS: 0,
		IwkN26:  0,
		Mcsi:    0,
		Mpsi:    0,
	}
	self.UriScheme = models.UriScheme_HTTP
	self.SupportDnnLists = []string{config.DNN}
	security := &context.Security{
		IntegrityOrder: []string{"NIA1", "NIA2"},
		CipheringOrder: []string{"NEA0"},
	}
	self.SecurityAlgorithm.IntegrityOrder = getIntAlgOrder(security.IntegrityOrder)
	self.SecurityAlgorithm.CipheringOrder = getEncAlgOrder(security.CipheringOrder)
	self.NetworkName = context.NetworkName{
		Full:  "SDCORE5G",
		Short: "SDCORE",
	}
	self.T3502Value = 720
	self.T3512Value = 3600
	self.T3513Cfg = context.TimerValue{
		Enable:        true,
		ExpireTime:    6 * time.Second,
		MaxRetryTimes: 4,
	}
	self.T3522Cfg = context.TimerValue{
		Enable:        true,
		ExpireTime:    6 * time.Second,
		MaxRetryTimes: 4,
	}
	self.T3550Cfg = context.TimerValue{
		Enable:        true,
		ExpireTime:    6 * time.Second,
		MaxRetryTimes: 4,
	}
	self.T3560Cfg = context.TimerValue{
		Enable:        true,
		ExpireTime:    6 * time.Second,
		MaxRetryTimes: 4,
	}
	self.T3565Cfg = context.TimerValue{
		Enable:        true,
		ExpireTime:    6 * time.Second,
		MaxRetryTimes: 4,
	}
	self.DbInstance = dbInstance

	StartNGAPService()
	return nil
}

func getIntAlgOrder(integrityOrder []string) (intOrder []uint8) {
	for _, intAlg := range integrityOrder {
		switch intAlg {
		case "NIA0":
			intOrder = append(intOrder, security.AlgIntegrity128NIA0)
		case "NIA1":
			intOrder = append(intOrder, security.AlgIntegrity128NIA1)
		case "NIA2":
			intOrder = append(intOrder, security.AlgIntegrity128NIA2)
		case "NIA3":
			intOrder = append(intOrder, security.AlgIntegrity128NIA3)
		default:
			logger.AmfLog.Errorf("Unsupported algorithm: %s", intAlg)
		}
	}
	return
}

func getEncAlgOrder(cipheringOrder []string) (encOrder []uint8) {
	for _, encAlg := range cipheringOrder {
		switch encAlg {
		case "NEA0":
			encOrder = append(encOrder, security.AlgCiphering128NEA0)
		case "NEA1":
			encOrder = append(encOrder, security.AlgCiphering128NEA1)
		case "NEA2":
			encOrder = append(encOrder, security.AlgCiphering128NEA2)
		case "NEA3":
			encOrder = append(encOrder, security.AlgCiphering128NEA3)
		default:
			logger.AmfLog.Errorf("Unsupported algorithm: %s", encAlg)
		}
	}
	return
}

func StartNGAPService() {
	self := context.AMF_Self()

	ngapHandler := service.NGAPHandler{
		HandleMessage:      ngap.Dispatch,
		HandleNotification: ngap.HandleSCTPNotification,
	}
	service.Run(self.NgapIpList, self.NgapPort, ngapHandler)

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signalChannel
		Terminate()
		os.Exit(0)
	}()
}

// Used in AMF planned removal procedure
func Terminate() {
	logger.AmfLog.Infof("Terminating AMF...")
	amfSelf := context.AMF_Self()

	// send AMF status indication to ran to notify ran that this AMF will be unavailable
	logger.AmfLog.Infof("Send AMF Status Indication to Notify RANs due to AMF terminating")
	guamiList := context.GetServedGuamiList()
	unavailableGuamiList := message.BuildUnavailableGUAMIList(guamiList)
	amfSelf.AmfRanPool.Range(func(key, value interface{}) bool {
		ran := value.(*context.AmfRan)
		message.SendAMFStatusIndication(ran, unavailableGuamiList)
		return true
	})

	service.Stop()

	logger.AmfLog.Infof("AMF terminated")
}
