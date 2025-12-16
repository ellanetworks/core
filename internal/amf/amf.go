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
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/security"
	"go.uber.org/zap"
)

func Start(dbInstance *db.Database, n2Address string, n2Port int) error {
	self := context.AMFSelf()
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
	self.SecurityAlgorithm.IntegrityOrder = []uint8{
		security.AlgIntegrity128NIA2,
		security.AlgIntegrity128NIA1,
		security.AlgIntegrity128NIA0,
	}
	self.SecurityAlgorithm.CipheringOrder = []uint8{
		security.AlgCiphering128NEA2,
		security.AlgCiphering128NEA1,
		security.AlgCiphering128NEA0,
	}
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
	self.Name = "amf"
	self.RelativeCapacity = 0xff

	err := StartNGAPService(n2Address, n2Port)
	if err != nil {
		return fmt.Errorf("failed to start NGAP service: %+v", err)
	}
	return nil
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

	ctx := ctxt.Background()

	operatorInfo, err := context.GetOperatorInfo(ctxt.Background())
	if err != nil {
		logger.AmfLog.Error("Could not get operator info", zap.Error(err))
		return
	}

	unavailableGuamiList := message.BuildUnavailableGUAMIList(operatorInfo.Guami)

	for _, ran := range amfSelf.AmfRanPool {
		err := message.SendAMFStatusIndication(ctx, ran, unavailableGuamiList)
		if err != nil {
			logger.AmfLog.Error("failed to send AMF Status Indication to RAN", zap.Error(err))
		}
	}

	service.Stop()

	logger.AmfLog.Info("AMF terminated")
}
