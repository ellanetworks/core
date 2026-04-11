package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleNGReset(ctx context.Context, ran *amf.Radio, msg decode.NGReset) {
	logger.WithTrace(ctx, ran.Log).Debug("Received NG Reset with Cause", logger.Cause(causeToString(msg.Cause)))

	switch msg.ResetType.Present {
	case ngapType.ResetTypePresentNGInterface:
		logger.WithTrace(ctx, ran.Log).Debug("ResetType Present: NG Interface")
		ran.RemoveAllUeInRan()
		logger.WithTrace(ctx, ran.Log).Debug("All UE Context in RAN have been removed")

		err := ran.NGAPSender.SendNGResetAcknowledge(ctx, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending NG Reset Acknowledge", zap.Error(err))
			return
		}
	case ngapType.ResetTypePresentPartOfNGInterface:
		logger.WithTrace(ctx, ran.Log).Debug("ResetType Present: Part of NG Interface")

		partOfNGInterface := msg.ResetType.PartOfNGInterface
		if partOfNGInterface == nil {
			logger.WithTrace(ctx, ran.Log).Error("PartOfNGInterface is nil")
			return
		}

		var ranUe *amf.RanUe

		for _, ueAssociatedLogicalNGConnectionItem := range partOfNGInterface.List {
			if ueAssociatedLogicalNGConnectionItem.AMFUENGAPID != nil {
				logger.WithTrace(ctx, ran.Log).Debug("NG Reset with AMFUENGAPID", zap.Int64("AmfUeNgapID", ueAssociatedLogicalNGConnectionItem.AMFUENGAPID.Value))
				ranUe = ran.FindUEByAmfUeNgapID(ueAssociatedLogicalNGConnectionItem.AMFUENGAPID.Value)
			} else if ueAssociatedLogicalNGConnectionItem.RANUENGAPID != nil {
				logger.WithTrace(ctx, ran.Log).Debug("NG Reset with RANUENGAPID", zap.Int64("RanUeNgapID", ueAssociatedLogicalNGConnectionItem.RANUENGAPID.Value))
				ranUe = ran.FindUEByRanUeNgapID(ueAssociatedLogicalNGConnectionItem.RANUENGAPID.Value)
			}

			if ranUe == nil {
				logger.WithTrace(ctx, ran.Log).Warn("Cannot not find UE Context")

				if ueAssociatedLogicalNGConnectionItem.AMFUENGAPID != nil {
					logger.WithTrace(ctx, ran.Log).Warn("AMFUENGAPID is not empty", zap.Int64("AmfUeNgapID", ueAssociatedLogicalNGConnectionItem.AMFUENGAPID.Value))
				}

				if ueAssociatedLogicalNGConnectionItem.RANUENGAPID != nil {
					logger.WithTrace(ctx, ran.Log).Warn("RANUENGAPID is not empty", zap.Int64("RanUeNgapID", ueAssociatedLogicalNGConnectionItem.RANUENGAPID.Value))
				}

				continue
			}

			err := ranUe.Remove()
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
			}
		}

		err := ran.NGAPSender.SendNGResetAcknowledge(ctx, partOfNGInterface)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending NG Reset Acknowledge", zap.Error(err))
			return
		}
	default:
		logger.WithTrace(ctx, ran.Log).Warn("Invalid ResetType", zap.Any("ResetType", msg.ResetType.Present))
	}
}
