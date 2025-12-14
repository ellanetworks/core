package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/consumer"
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverRequired(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var handoverType *ngapType.HandoverType
	var cause *ngapType.Cause
	var targetID *ngapType.TargetID
	var pDUSessionResourceListHORqd *ngapType.PDUSessionResourceListHORqd
	var sourceToTargetTransparentContainer *ngapType.SourceToTargetTransparentContainer
	var iesCriticalityDiagnostics ngapType.CriticalityDiagnosticsIEList

	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	initiatingMessage := msg.InitiatingMessage
	if initiatingMessage == nil {
		ran.Log.Error("Initiating Message is nil")
		return
	}

	HandoverRequired := initiatingMessage.Value.HandoverRequired
	if HandoverRequired == nil {
		ran.Log.Error("HandoverRequired is nil")
		return
	}

	for i := 0; i < len(HandoverRequired.ProtocolIEs.List); i++ {
		ie := HandoverRequired.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID // reject
		case ngapType.ProtocolIEIDRANUENGAPID: // reject
			rANUENGAPID = ie.Value.RANUENGAPID
		case ngapType.ProtocolIEIDHandoverType: // reject
			handoverType = ie.Value.HandoverType
		case ngapType.ProtocolIEIDCause: // ignore
			cause = ie.Value.Cause
		case ngapType.ProtocolIEIDTargetID: // reject
			targetID = ie.Value.TargetID
		case ngapType.ProtocolIEIDPDUSessionResourceListHORqd: // reject
			pDUSessionResourceListHORqd = ie.Value.PDUSessionResourceListHORqd
		case ngapType.ProtocolIEIDSourceToTargetTransparentContainer: // reject
			sourceToTargetTransparentContainer = ie.Value.SourceToTargetTransparentContainer
		}
	}

	if aMFUENGAPID == nil {
		ran.Log.Error("AmfUeNgapID is nil")
		item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDAMFUENGAPID)
		iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
	}
	if rANUENGAPID == nil {
		ran.Log.Error("RanUeNgapID is nil")
		item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDRANUENGAPID)
		iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
	}

	if handoverType == nil {
		ran.Log.Error("handoverType is nil")
		item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDHandoverType)
		iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
	}
	if targetID == nil {
		ran.Log.Error("targetID is nil")
		item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDTargetID)
		iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
	}
	if pDUSessionResourceListHORqd == nil {
		ran.Log.Error("pDUSessionResourceListHORqd is nil")
		item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDPDUSessionResourceListHORqd)
		iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
	}
	if sourceToTargetTransparentContainer == nil {
		ran.Log.Error("sourceToTargetTransparentContainer is nil")
		item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDSourceToTargetTransparentContainer)
		iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
	}

	if len(iesCriticalityDiagnostics.List) > 0 {
		procedureCode := ngapType.ProcedureCodeHandoverPreparation
		triggeringMessage := ngapType.TriggeringMessagePresentInitiatingMessage
		procedureCriticality := ngapType.CriticalityPresentReject
		criticalityDiagnostics := buildCriticalityDiagnostics(&procedureCode, &triggeringMessage,
			&procedureCriticality, &iesCriticalityDiagnostics)
		err := message.SendErrorIndication(ctx, ran, nil, nil, nil, &criticalityDiagnostics)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}
		ran.Log.Info("sent error indication")
		return
	}

	sourceUe := ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
	if sourceUe == nil {
		ran.Log.Error("Cannot find UE", zap.Int64("RAN_UE_NGAP_ID", rANUENGAPID.Value))
		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}
		err := message.SendErrorIndication(ctx, ran, nil, nil, &cause, nil)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err), zap.Int64("RAN_UE_NGAP_ID", rANUENGAPID.Value))
			return
		}
		ran.Log.Info("sent error indication to source UE")
		return
	}

	amfUe := sourceUe.AmfUe
	if amfUe == nil {
		ran.Log.Error("Cannot find amfUE from sourceUE")
		return
	}

	if targetID.Present != ngapType.TargetIDPresentTargetRANNodeID {
		ran.Log.Error("targetID type is not supported", zap.Int("targetID", targetID.Present))
		return
	}

	amfUe.SetOnGoing(&context.OnGoingProcedureWithPrio{
		Procedure: context.OnGoingProcedureN2Handover,
	})

	if !amfUe.SecurityContextIsValid() {
		sourceUe.Log.Info("handle Handover Preparation Failure [Authentication Failure]")
		cause = &ngapType.Cause{
			Present: ngapType.CausePresentNas,
			Nas: &ngapType.CauseNas{
				Value: ngapType.CauseNasPresentAuthenticationFailure,
			},
		}
		err := message.SendHandoverPreparationFailure(ctx, sourceUe, *cause, nil)
		if err != nil {
			sourceUe.Log.Error("error sending handover preparation failure", zap.Error(err))
			return
		}
		sourceUe.Log.Info("sent handover preparation failure to source UE")
		return
	}
	aMFSelf := context.AMFSelf()
	targetRanNodeID := util.RanIDToModels(targetID.TargetRANNodeID.GlobalRANNodeID)
	targetRan, ok := aMFSelf.AmfRanFindByRanID(targetRanNodeID)
	if !ok {
		// handover between different AMF
		sourceUe.Log.Warn("Handover required : cannot find target Ran Node Id in this AMF. Handover between different AMF has not been implemented yet", zap.Any("targetRanNodeID", targetRanNodeID))
		return
		// Described in (23.502 4.9.1.3.2) step 3.Namf_Communication_CreateUEContext Request
	}

	// Handover in same AMF
	sourceUe.HandOverType.Value = handoverType.Value

	var pduSessionReqList ngapType.PDUSessionResourceSetupListHOReq
	for _, pDUSessionResourceHoItem := range pDUSessionResourceListHORqd.List {
		pduSessionIDInt32 := int32(pDUSessionResourceHoItem.PDUSessionID.Value)
		if smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionIDInt32); exist {
			response, err := consumer.SendUpdateSmContextN2HandoverPreparing(ctx, amfUe, smContext,
				models.N2SmInfoTypeHandoverRequired, pDUSessionResourceHoItem.HandoverRequiredTransfer)
			if err != nil {
				sourceUe.Log.Error("SendUpdateSmContextN2HandoverPreparing Error", zap.Error(err), zap.Int32("PduSessionID", pduSessionIDInt32))
			}
			if response == nil {
				sourceUe.Log.Error("SendUpdateSmContextN2HandoverPreparing Error for PduSessionID", zap.Int32("PduSessionID", pduSessionIDInt32))
				continue
			} else if response.BinaryDataN2SmInformation != nil {
				message.AppendPDUSessionResourceSetupListHOReq(&pduSessionReqList, pduSessionIDInt32,
					smContext.Snssai(), response.BinaryDataN2SmInformation)
			}
		}
	}
	if len(pduSessionReqList.List) == 0 {
		sourceUe.Log.Info("handle Handover Preparation Failure [HoFailure In Target5GC NgranNode Or TargetSystem]")
		cause = &ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
			},
		}
		err := message.SendHandoverPreparationFailure(ctx, sourceUe, *cause, nil)
		if err != nil {
			sourceUe.Log.Error("error sending handover preparation failure", zap.Error(err))
			return
		}
		sourceUe.Log.Info("sent handover preparation failure to source UE")
		return
	}
	amfUe.UpdateNH()
	operatorInfo, err := context.GetOperatorInfo(ctx)
	if err != nil {
		sourceUe.Log.Error("Could not get operator info", zap.Error(err))
	}
	err = message.SendHandoverRequest(ctx, sourceUe, targetRan, *cause, pduSessionReqList, *sourceToTargetTransparentContainer, operatorInfo.SupportedPLMN, operatorInfo.Guami)
	if err != nil {
		sourceUe.Log.Error("error sending handover request to target UE", zap.Error(err))
		return
	}
	sourceUe.Log.Info("sent handover request to target UE")
}
