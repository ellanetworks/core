package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverRequestAcknowledge(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	successfulOutcome := msg.SuccessfulOutcome
	if successfulOutcome == nil {
		ran.Log.Error("SuccessfulOutcome is nil")
		return
	}

	handoverRequestAcknowledge := successfulOutcome.Value.HandoverRequestAcknowledge // reject
	if handoverRequestAcknowledge == nil {
		ran.Log.Error("HandoverRequestAcknowledge is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var pDUSessionResourceAdmittedList *ngapType.PDUSessionResourceAdmittedList
	var pDUSessionResourceFailedToSetupListHOAck *ngapType.PDUSessionResourceFailedToSetupListHOAck
	var targetToSourceTransparentContainer *ngapType.TargetToSourceTransparentContainer
	var iesCriticalityDiagnostics ngapType.CriticalityDiagnosticsIEList

	for _, ie := range handoverRequestAcknowledge.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID: // ignore
			aMFUENGAPID = ie.Value.AMFUENGAPID
		case ngapType.ProtocolIEIDRANUENGAPID: // ignore
			rANUENGAPID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDPDUSessionResourceAdmittedList: // ignore
			pDUSessionResourceAdmittedList = ie.Value.PDUSessionResourceAdmittedList
		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListHOAck: // ignore
			pDUSessionResourceFailedToSetupListHOAck = ie.Value.PDUSessionResourceFailedToSetupListHOAck
		case ngapType.ProtocolIEIDTargetToSourceTransparentContainer: // reject
			targetToSourceTransparentContainer = ie.Value.TargetToSourceTransparentContainer
		}
	}
	if targetToSourceTransparentContainer == nil {
		ran.Log.Error("TargetToSourceTransparentContainer is nil")
		item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDTargetToSourceTransparentContainer)
		iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
	}
	if len(iesCriticalityDiagnostics.List) > 0 {
		ran.Log.Error("Has missing reject IE(s)")

		procedureCode := ngapType.ProcedureCodeHandoverResourceAllocation
		triggeringMessage := ngapType.TriggeringMessagePresentSuccessfulOutcome
		procedureCriticality := ngapType.CriticalityPresentReject
		criticalityDiagnostics := buildCriticalityDiagnostics(&procedureCode, &triggeringMessage, &procedureCriticality, &iesCriticalityDiagnostics)
		err := message.SendErrorIndication(ctx, ran, nil, nil, nil, &criticalityDiagnostics)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}
		ran.Log.Info("sent error indication")
	}

	targetUe := context.AMFSelf().RanUeFindByAmfUeNgapID(aMFUENGAPID.Value)
	if targetUe == nil {
		ran.Log.Error("No UE Context", zap.Int64("AmfUeNgapID", aMFUENGAPID.Value), zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		return
	}

	if rANUENGAPID != nil {
		targetUe.RanUeNgapID = rANUENGAPID.Value
	}

	targetUe.Ran = ran
	ran.Log.Debug("Handle Handover Request Acknowledge", zap.Any("RanUeNgapID", targetUe.RanUeNgapID), zap.Any("AmfUeNgapID", targetUe.AmfUeNgapID))

	amfUe := targetUe.AmfUe
	if amfUe == nil {
		targetUe.Log.Error("amfUe is nil")
		return
	}

	var pduSessionResourceHandoverList ngapType.PDUSessionResourceHandoverList
	var pduSessionResourceToReleaseList ngapType.PDUSessionResourceToReleaseListHOCmd

	// describe in 23.502 4.9.1.3.2 step11
	if pDUSessionResourceAdmittedList != nil {
		for _, item := range pDUSessionResourceAdmittedList.List {
			pduSessionID := item.PDUSessionID.Value
			transfer := item.HandoverRequestAcknowledgeTransfer
			pduSessionIDUint8 := uint8(pduSessionID)
			if smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionIDUint8); exist {
				n2Rsp, err := pdusession.UpdateSmContextN2HandoverPrepared(smContext.SmContextRef(), transfer)
				if err != nil {
					targetUe.Log.Error("Send HandoverRequestAcknowledgeTransfer error", zap.Error(err))
					continue
				}
				handoverItem := ngapType.PDUSessionResourceHandoverItem{}
				handoverItem.PDUSessionID = item.PDUSessionID
				handoverItem.HandoverCommandTransfer = n2Rsp
				pduSessionResourceHandoverList.List = append(pduSessionResourceHandoverList.List, handoverItem)
				targetUe.SuccessPduSessionID = append(targetUe.SuccessPduSessionID, pduSessionIDUint8)
			}
		}
	}

	if pDUSessionResourceFailedToSetupListHOAck != nil {
		for _, item := range pDUSessionResourceFailedToSetupListHOAck.List {
			pduSessionID := item.PDUSessionID.Value
			transfer := item.HandoverResourceAllocationUnsuccessfulTransfer
			pduSessionIDUint8 := uint8(pduSessionID)
			if smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionIDUint8); exist {
				_, err := pdusession.UpdateSmContextN2HandoverPrepared(smContext.SmContextRef(), transfer)
				if err != nil {
					targetUe.Log.Error("Send HandoverResourceAllocationUnsuccessfulTransfer error", zap.Error(err))
				}
			}
		}
	}

	sourceUe := targetUe.SourceUe
	if sourceUe == nil {
		ran.Log.Error("handover between different Ue has not been implement yet")
	} else {
		ran.Log.Debug("handle handover request acknowledge", zap.Int64("sourceRanUeNgapID", sourceUe.RanUeNgapID), zap.Int64("sourceAmfUeNgapID", sourceUe.AmfUeNgapID),
			zap.Int64("targetRanUeNgapID", targetUe.RanUeNgapID), zap.Int64("targetAmfUeNgapID", targetUe.AmfUeNgapID))
		if len(pduSessionResourceHandoverList.List) == 0 {
			targetUe.Log.Info("handle Handover Preparation Failure [HoFailure In Target5GC NgranNode Or TargetSystem]")
			cause := &ngapType.Cause{
				Present: ngapType.CausePresentRadioNetwork,
				RadioNetwork: &ngapType.CauseRadioNetwork{
					Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
				},
			}
			err := message.SendHandoverPreparationFailure(ctx, sourceUe, *cause, nil)
			if err != nil {
				ran.Log.Error("error sending handover preparation failure", zap.Error(err))
			}
			ran.Log.Info("sent handover preparation failure to source UE")
			return
		}
		err := message.SendHandoverCommand(ctx, sourceUe, pduSessionResourceHandoverList, pduSessionResourceToReleaseList, *targetToSourceTransparentContainer, nil)
		if err != nil {
			ran.Log.Error("error sending handover command to source UE", zap.Error(err))
		}
		ran.Log.Info("sent handover command to source UE")
	}
}
