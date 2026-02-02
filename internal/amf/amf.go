// Copyright 2024 Ella Networks

package amf

import (
	"context"
	"fmt"
	"time"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/ngap/service"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	nasLogger "github.com/free5gc/nas/logger"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/security"
	"go.uber.org/zap"
)

func Start(ctx context.Context, dbInstance *db.Database, n2Address string, n2Port int, smf amfContext.SmfSbi) error {
	nasLogger.SetLogLevel(0) // Panic level to avoid NAS log output

	self := amfContext.AMFSelf()
	self.Smf = smf
	self.NetworkFeatureSupport5GS = &amfContext.NetworkFeatureSupport5GS{
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
	self.NetworkName = amfContext.NetworkName{
		Full:  "ELLACORE5G",
		Short: "ELLACORE",
	}
	self.TimeZone = nasConvert.GetTimeZone(time.Now())
	self.T3502Value = 720
	self.T3512Value = 3600
	self.T3513Cfg = amfContext.TimerValue{
		Enable:        true,
		ExpireTime:    6 * time.Second,
		MaxRetryTimes: 4,
	}
	self.T3522Cfg = amfContext.TimerValue{
		Enable:        true,
		ExpireTime:    6 * time.Second,
		MaxRetryTimes: 4,
	}
	self.T3550Cfg = amfContext.TimerValue{
		Enable:        true,
		ExpireTime:    6 * time.Second,
		MaxRetryTimes: 4,
	}
	self.T3555Cfg = amfContext.TimerValue{
		Enable:        true,
		ExpireTime:    6 * time.Second,
		MaxRetryTimes: 4,
	}
	self.T3560Cfg = amfContext.TimerValue{
		Enable:        true,
		ExpireTime:    6 * time.Second,
		MaxRetryTimes: 4,
	}
	self.T3565Cfg = amfContext.TimerValue{
		Enable:        true,
		ExpireTime:    6 * time.Second,
		MaxRetryTimes: 4,
	}
	self.DBInstance = dbInstance
	self.Name = "amf"
	self.RelativeCapacity = 0xff

	err := service.Run(ctx, n2Address, n2Port)
	if err != nil {
		return fmt.Errorf("failed to start NGAP service: %+v", err)
	}

	return nil
}

func Close(ctx context.Context) {
	amf := amfContext.AMFSelf()

	operatorInfo, err := amf.GetOperatorInfo(ctx)
	if err != nil {
		logger.AmfLog.Error("Could not get operator info", zap.Error(err))
		return
	}

	unavailableGuamiList := send.BuildUnavailableGUAMIList(operatorInfo.Guami)

	for _, ran := range amf.Radios {
		err := ran.NGAPSender.SendAMFStatusIndication(ctx, unavailableGuamiList)
		if err != nil {
			logger.AmfLog.Error("failed to send AMF Status Indication to RAN", zap.Error(err))
		}
	}

	service.Stop()

	logger.AmfLog.Info("AMF terminated")
}
