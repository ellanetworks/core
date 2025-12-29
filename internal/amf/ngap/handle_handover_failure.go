package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverFailure(ctx context.Context, ran *amfContext.AmfRan, message *ngapType.NGAPPDU) {
	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if message == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	unsuccessfulOutcome := message.UnsuccessfulOutcome // reject
	if unsuccessfulOutcome == nil {
		ran.Log.Error("Unsuccessful Message is nil")
		return
	}

	handoverFailure := unsuccessfulOutcome.Value.HandoverFailure
	if handoverFailure == nil {
		ran.Log.Error("HandoverFailure is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var cause *ngapType.Cause
	var targetUe *amfContext.RanUe
	var criticalityDiagnostics *ngapType.CriticalityDiagnostics

	for _, ie := range handoverFailure.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID: // ignore
			aMFUENGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDCause: // ignore
			cause = ie.Value.Cause
		case ngapType.ProtocolIEIDCriticalityDiagnostics: // ignore
			criticalityDiagnostics = ie.Value.CriticalityDiagnostics
		}
	}

	causePresent := ngapType.CausePresentRadioNetwork
	causeValue := ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem
	var err error
	if cause != nil {
		ran.Log.Debug("Handover Failure Cause", zap.String("Cause", causeToString(*cause)))
		causePresent, causeValue, err = getCause(cause)
		if err != nil {
			ran.Log.Error("Get Cause from Handover Failure Error", zap.Error(err))
			return
		}
	}

	targetUe = amfContext.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)

	if targetUe == nil {
		ran.Log.Error("No UE Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}
		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}
		ran.Log.Info("sent error indication")
		return
	}

	targetUe.Ran = ran
	sourceUe := targetUe.SourceUe
	if sourceUe == nil {
		ran.Log.Error("N2 Handover between AMF has not been implemented yet")
	} else {
		sourceUe.AmfUe.SetOnGoing(&amfContext.OnGoingProcedureWithPrio{
			Procedure: amfContext.OnGoingProcedureNothing,
		})
		err := sourceUe.Ran.NGAPSender.SendHandoverPreparationFailure(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, *cause, criticalityDiagnostics)
		if err != nil {
			ran.Log.Error("error sending handover preparation failure", zap.Error(err))
			return
		}
		ran.Log.Info("sent handover preparation failure to source UE")
	}

	targetUe.ReleaseAction = amfContext.UeContextReleaseHandover

	err = targetUe.Ran.NGAPSender.SendUEContextReleaseCommand(ctx, targetUe.AmfUeNgapID, targetUe.RanUeNgapID, causePresent, causeValue)
	if err != nil {
		ran.Log.Error("error sending UE Context Release Command to target UE", zap.Error(err))
		return
	}
}
