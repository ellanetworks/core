package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverCancel(ctx context.Context, ran *amf.Radio, msg decode.HandoverCancel) {
	sourceUe := ran.FindUEByRanUeNgapID(msg.RANUENGAPID)
	if sourceUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("RanUeNgapID", msg.RANUENGAPID))

		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}

		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err), zap.Int64("RAN_UE_NGAP_ID", msg.RANUENGAPID))
			return
		}

		logger.WithTrace(ctx, ran.Log).Info("sent error indication to source UE")

		return
	}

	if sourceUe.AmfUeNgapID != msg.AMFUENGAPID {
		logger.WithTrace(ctx, ran.Log).Warn("Conflict AMF_UE_NGAP_ID", zap.Int64("sourceUe.AmfUeNgapID", sourceUe.AmfUeNgapID), zap.Int64("aMFUENGAPID.Value", msg.AMFUENGAPID))
	}

	logger.WithTrace(ctx, ran.Log).Debug("Handle Handover Cancel", zap.Int64("sourceRanUeNgapID", sourceUe.RanUeNgapID), zap.Int64("sourceAmfUeNgapID", sourceUe.AmfUeNgapID))
	sourceUe.TouchLastSeen()

	causePresent := ngapType.CausePresentRadioNetwork
	causeValue := ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem

	var err error

	if msg.Cause != nil {
		logger.WithTrace(ctx, ran.Log).Debug("Handover Cancel Cause", logger.Cause(causeToString(*msg.Cause)))

		causePresent, causeValue, err = getCause(msg.Cause)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("Get Cause from Handover Failure Error", zap.Error(err))
			return
		}
	}

	targetUe := sourceUe.TargetUe
	if targetUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("N2 Handover between AMF has not been implemented yet")
		return
	}

	targetUe.ReleaseAction = amf.UeContextReleaseHandover

	err = targetUe.SendUEContextReleaseCommand(ctx, causePresent, causeValue)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending UE Context Release Command to target UE", zap.Error(err))
		return
	}

	err = sourceUe.SendHandoverCancelAcknowledge(ctx)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending handover cancel acknowledge to source UE", zap.Error(err))
		return
	}

	logger.WithTrace(ctx, ran.Log).Info("sent handover cancel acknowledge to source UE")
}
