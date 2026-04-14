package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverNotify(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.HandoverNotify) {
	targetUe := ran.FindUEByRanUeNgapID(msg.RANUENGAPID)
	if targetUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No RanUe Context", zap.Int64("AmfUeNgapID", msg.AMFUENGAPID), zap.Int64("RanUeNgapID", msg.RANUENGAPID))

		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}

		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
			return
		}

		return
	}

	if msg.UserLocationInformation != nil {
		targetUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
	}

	amfUe := targetUe.AmfUe()
	if amfUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("AmfUe is nil")
		return
	}

	sourceUe := targetUe.SourceUe
	if sourceUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("N2 Handover between AMF has not been implemented yet")
		return
	}

	logger.WithTrace(ctx, targetUe.Log).Info("Handle Handover notification Finshed ")

	amfUe.Procedures.End(procedure.N2Handover)
	amfUe.AttachRanUe(targetUe)

	sourceUe.ReleaseAction = amf.UeContextReleaseHandover

	err := sourceUe.Radio.NGAPSender.SendUEContextReleaseCommand(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		logger.WithTrace(ctx, targetUe.Log).Error("error sending ue context release command", zap.Error(err))
		return
	}
}
