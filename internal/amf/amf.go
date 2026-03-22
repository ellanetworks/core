// Copyright 2024 Ella Networks

package amf

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/ngap/service"
	"github.com/ellanetworks/core/internal/logger"
	nasLogger "github.com/free5gc/nas/logger"
	"go.uber.org/zap"
)

func Start(ctx context.Context, amf *amfContext.AMF, n2Address string, n2Port int) (*service.Server, error) {
	nasLogger.SetLogLevel(0) // Panic level to avoid NAS log output

	srv := service.NewServer(amf)

	err := srv.ListenAndServe(ctx, n2Address, n2Port)
	if err != nil {
		return nil, fmt.Errorf("failed to start NGAP service: %+v", err)
	}

	return srv, nil
}

func Close(ctx context.Context, amf *amfContext.AMF, srv *service.Server) {
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

	srv.Shutdown(ctx)

	logger.AmfLog.Info("AMF terminated")
}
