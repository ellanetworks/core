package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverCancel(ctx context.Context, ran *amf.Radio, msg *ngapType.HandoverCancel) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID *ngapType.AMFUENGAPID
		rANUENGAPID *ngapType.RANUENGAPID
		cause       *ngapType.Cause
	)

	for i := 0; i < len(msg.ProtocolIEs.List); i++ {
		ie := msg.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("AMFUENGAPID is nil")
				return
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID is nil")
				return
			}
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
			if cause == nil {
				logger.WithTrace(ctx, ran.Log).Error("Cause is nil")
				return
			}
		}
	}

	if aMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMFUENGAPID IE (mandatory) is missing in HandoverCancel")
		return
	}

	if rANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID IE (mandatory) is missing in HandoverCancel")
		return
	}

	sourceUe := ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	if sourceUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}

		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err), zap.Int64("RAN_UE_NGAP_ID", rANUENGAPID.Value))
			return
		}

		logger.WithTrace(ctx, ran.Log).Info("sent error indication to source UE")

		return
	}

	if sourceUe.AmfUeNgapID != aMFUENGAPID.Value {
		logger.WithTrace(ctx, ran.Log).Warn("Conflict AMF_UE_NGAP_ID", zap.Int64("sourceUe.AmfUeNgapID", sourceUe.AmfUeNgapID), zap.Int64("aMFUENGAPID.Value", aMFUENGAPID.Value))
	}

	logger.WithTrace(ctx, ran.Log).Debug("Handle Handover Cancel", zap.Int64("sourceRanUeNgapID", sourceUe.RanUeNgapID), zap.Int64("sourceAmfUeNgapID", sourceUe.AmfUeNgapID))
	sourceUe.TouchLastSeen()

	causePresent := ngapType.CausePresentRadioNetwork
	causeValue := ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem

	var err error

	if cause != nil {
		logger.WithTrace(ctx, ran.Log).Debug("Handover Cancel Cause", logger.Cause(causeToString(*cause)))

		causePresent, causeValue, err = getCause(cause)
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
