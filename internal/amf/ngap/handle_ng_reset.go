package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleNGReset(ctx context.Context, ran *amfContext.Radio, msg *ngapType.NGReset) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		cause     *ngapType.Cause
		resetType *ngapType.ResetType
	)

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
			if cause == nil {
				logger.WithTrace(ctx, ran.Log).Error("Cause is nil")
				return
			}
		case ngapType.ProtocolIEIDResetType:
			resetType = ie.Value.ResetType
			if resetType == nil {
				logger.WithTrace(ctx, ran.Log).Error("ResetType is nil")
				return
			}
		}
	}

	if cause == nil {
		logger.WithTrace(ctx, logger.AmfLog).Error("Cause IE (mandatory) is missing in NG Reset")
		return
	}

	if resetType == nil {
		logger.WithTrace(ctx, logger.AmfLog).Error("ResetType IE (mandatory) is missing in NG Reset")
		return
	}

	logger.WithTrace(ctx, logger.AmfLog).Debug("Received NG Reset with Cause", logger.Cause(causeToString(*cause)))

	switch resetType.Present {
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

		partOfNGInterface := resetType.PartOfNGInterface
		if partOfNGInterface == nil {
			logger.WithTrace(ctx, ran.Log).Error("PartOfNGInterface is nil")
			return
		}

		var ranUe *amfContext.RanUe

		for _, ueAssociatedLogicalNGConnectionItem := range partOfNGInterface.List {
			if ueAssociatedLogicalNGConnectionItem.AMFUENGAPID != nil {
				logger.WithTrace(ctx, ran.Log).Debug("NG Reset with AMFUENGAPID", zap.Int64("AmfUeNgapID", ueAssociatedLogicalNGConnectionItem.AMFUENGAPID.Value))

				for _, ue := range ran.RanUEs {
					if ue.AmfUeNgapID == ueAssociatedLogicalNGConnectionItem.AMFUENGAPID.Value {
						ranUe = ue
						break
					}
				}
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
				logger.WithTrace(ctx, ran.Log).Error(err.Error())
			}
		}

		err := ran.NGAPSender.SendNGResetAcknowledge(ctx, partOfNGInterface)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending NG Reset Acknowledge", zap.Error(err))
			return
		}
	default:
		logger.WithTrace(ctx, ran.Log).Warn("Invalid ResetType", zap.Any("ResetType", resetType.Present))
	}
}
