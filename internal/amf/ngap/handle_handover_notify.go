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
	targetUe, ok := resolveUE(ctx, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
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

	amfUe.NasConn().Procedures.End(procedure.N2Handover)
	amfUe.AttachRanUe(targetUe)

	sourceUe.ReleaseAction = amf.UeContextReleaseHandover

	err := sourceUe.Radio().NGAPSender.SendUEContextReleaseCommand(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, ngapType.CausePresentNas, ngapType.CauseNasPresentNormalRelease)
	if err != nil {
		logger.WithTrace(ctx, targetUe.Log).Error("error sending ue context release command", zap.Error(err))
		return
	}
}
