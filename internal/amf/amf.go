// Copyright 2024 Ella Networks

package amf

import (
	ctxt "context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/amf/ngap/service"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/security"
	"go.uber.org/zap"
)

func Start(dbInstance *db.Database, n2Address string, n2Port int) error {
	self := context.AMFSelf()
	self.NgapPort = n2Port
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
	self.URIScheme = models.URISchemeHTTP
	security := &context.Security{
		IntegrityOrder: []string{"NIA1", "NIA2"},
		CipheringOrder: []string{"NEA0"},
	}
	self.SecurityAlgorithm.IntegrityOrder = getIntAlgOrder(security.IntegrityOrder)
	self.SecurityAlgorithm.CipheringOrder = getEncAlgOrder(security.CipheringOrder)
	self.NetworkName = context.NetworkName{
		Full:  "ELLACORE5G",
		Short: "ELLACORE",
	}
	self.TimeZone = nasConvert.GetTimeZone(time.Now())
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
	self.T3555Cfg = context.TimerValue{
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
	self.DBInstance = dbInstance
	self.LadnPool = make(map[string]*context.LADN)
	self.Name = "amf"
	self.RelativeCapacity = 0xff

	err := StartNGAPService(n2Address, n2Port)
	if err != nil {
		return fmt.Errorf("failed to start NGAP service: %+v", err)
	}
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
			logger.AmfLog.Error("Unsupported algorithm", zap.String("algorithm", intAlg))
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
			logger.AmfLog.Error("Unsupported algorithm", zap.String("algorithm", encAlg))
		}
	}
	return
}

func StartNGAPService(ngapAddress string, ngapPort int) error {
	ngapHandler := service.NGAPHandler{
		HandleMessage:      ngap.Dispatch,
		HandleNotification: ngap.HandleSCTPNotification,
	}

	err := service.Run(ngapAddress, ngapPort, ngapHandler)
	if err != nil {
		return fmt.Errorf("failed to start NGAP service: %+v", err)
	}

	return nil
}

func Close() {
	amfSelf := context.AMFSelf()

	guamiList := context.GetServedGuamiList(ctxt.Background())
	unavailableGuamiList := message.BuildUnavailableGUAMIList(guamiList)
	amfSelf.AmfRanPool.Range(func(key, value interface{}) bool {
		ran := value.(*context.AmfRan)
		err := message.SendAMFStatusIndication(ran, unavailableGuamiList)
		if err != nil {
			logger.AmfLog.Error("failed to send AMF Status Indication to RAN", zap.Error(err))
		}
		return true
	})

	service.Stop()

	logger.AmfLog.Info("AMF terminated")
}
