package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandleUplinkRanConfigurationTransfer(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.UplinkRANConfigurationTransfer) {
	if msg.SONConfigurationTransferUL == nil {
		logger.WithTrace(ctx, ran.Log).Warn("sONConfigurationTransferUL is nil")
		return
	}

	targetRanNodeID := util.RanIDToModels(msg.SONConfigurationTransferUL.TargetRANNodeID.GlobalRANNodeID)

	if targetRanNodeID.GNbID != nil && targetRanNodeID.GNbID.GNBValue != "" {
		logger.WithTrace(ctx, ran.Log).Debug("targetRanID", zap.String("targetRanID", targetRanNodeID.GNbID.GNBValue))
	}

	targetRan, ok := amfInstance.FindRadioByRanID(targetRanNodeID)
	if !ok {
		logger.WithTrace(ctx, ran.Log).Warn("targetRan is nil")
		return
	}

	err := targetRan.NGAPSender.SendDownlinkRanConfigurationTransfer(ctx, msg.SONConfigurationTransferUL)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending downlink ran configuration transfer", zap.Error(err))
		return
	}

	logger.WithTrace(ctx, ran.Log).Info("sent downlink ran configuration transfer to target ran", zap.Any("RAN ID", targetRan.RanID))
}
