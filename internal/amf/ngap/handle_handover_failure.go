package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverFailure(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngapType.HandoverFailure) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID            *ngapType.AMFUENGAPID
		cause                  *ngapType.Cause
		targetUe               *amf.RanUe
		criticalityDiagnostics *ngapType.CriticalityDiagnostics
	)

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID: // ignore
			aMFUENGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDCause: // ignore
			cause = ie.Value.Cause
		case ngapType.ProtocolIEIDCriticalityDiagnostics: // ignore
			criticalityDiagnostics = ie.Value.CriticalityDiagnostics
		}
	}

	if aMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMFUENGAPID IE (mandatory) is missing in HandoverFailure")
		return
	}

	causePresent := ngapType.CausePresentRadioNetwork
	causeValue := ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem

	var err error

	if cause != nil {
		logger.WithTrace(ctx, ran.Log).Debug("Handover Failure Cause", logger.Cause(causeToString(*cause)))

		causePresent, causeValue, err = getCause(cause)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("Get Cause from Handover Failure Error", zap.Error(err))
			return
		}
	}

	targetUe = amfInstance.FindRanUeByAmfUeNgapID(aMFUENGAPID.Value)

	if targetUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
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

		logger.WithTrace(ctx, ran.Log).Info("sent error indication")

		return
	}

	targetUe.Radio = ran
	targetUe.TouchLastSeen()

	sourceUe := targetUe.SourceUe
	if sourceUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("N2 Handover between AMF has not been implemented yet")
	} else {
		if sourceAmfUe := sourceUe.AmfUe(); sourceAmfUe != nil {
			sourceAmfUe.SetOnGoing(amf.OnGoingProcedureNothing)
		}

		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
			},
		}
		if cause != nil {
			failureCause = *cause
		}

		if sourceUe.Radio == nil {
			logger.WithTrace(ctx, ran.Log).Error("source UE radio is nil, cannot send handover preparation failure")
		} else {
			err := sourceUe.Radio.NGAPSender.SendHandoverPreparationFailure(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, failureCause, criticalityDiagnostics)
			if err != nil {
				logger.WithTrace(ctx, ran.Log).Error("error sending handover preparation failure", zap.Error(err))
				return
			}

			logger.WithTrace(ctx, ran.Log).Info("sent handover preparation failure to source UE")
		}
	}

	targetUe.ReleaseAction = amf.UeContextReleaseHandover

	err = targetUe.Radio.NGAPSender.SendUEContextReleaseCommand(ctx, targetUe.AmfUeNgapID, targetUe.RanUeNgapID, causePresent, causeValue)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending UE Context Release Command to target UE", zap.Error(err))
		return
	}
}
