package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverRequired(ctx context.Context, ran *amfContext.AmfRan, msg *ngapType.HandoverRequired) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var handoverType *ngapType.HandoverType
	var cause *ngapType.Cause
	var targetID *ngapType.TargetID
	var pDUSessionResourceListHORqd *ngapType.PDUSessionResourceListHORqd
	var sourceToTargetTransparentContainer *ngapType.SourceToTargetTransparentContainer
	var iesCriticalityDiagnostics ngapType.CriticalityDiagnosticsIEList

	for i := 0; i < len(msg.ProtocolIEs.List); i++ {
		ie := msg.ProtocolIEs.List[i]
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
		criticalityDiagnostics := buildCriticalityDiagnostics(ngapType.ProcedureCodeHandoverPreparation, ngapType.TriggeringMessagePresentInitiatingMessage, ngapType.CriticalityPresentReject, &iesCriticalityDiagnostics)
		err := ran.NGAPSender.SendErrorIndication(ctx, nil, &criticalityDiagnostics)
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
		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
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

	amfUe.SetOnGoing(&amfContext.OnGoingProcedureWithPrio{
		Procedure: amfContext.OnGoingProcedureN2Handover,
	})

	if !amfUe.SecurityContextIsValid() {
		sourceUe.Log.Info("handle Handover Preparation Failure [Authentication Failure]")
		cause = &ngapType.Cause{
			Present: ngapType.CausePresentNas,
			Nas: &ngapType.CauseNas{
				Value: ngapType.CauseNasPresentAuthenticationFailure,
			},
		}
		sourceUe.AmfUe.SetOnGoing(&amfContext.OnGoingProcedureWithPrio{
			Procedure: amfContext.OnGoingProcedureNothing,
		})
		err := sourceUe.Ran.NGAPSender.SendHandoverPreparationFailure(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, *cause, nil)
		if err != nil {
			sourceUe.Log.Error("error sending handover preparation failure", zap.Error(err))
			return
		}
		sourceUe.Log.Info("sent handover preparation failure to source UE")
		return
	}
	aMFSelf := amfContext.AMFSelf()
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
		pduSessionIDUint8 := uint8(pDUSessionResourceHoItem.PDUSessionID.Value)
		if smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionIDUint8); exist {
			n2Rsp, err := pdusession.UpdateSmContextN2HandoverPreparing(smContext.Ref, pDUSessionResourceHoItem.HandoverRequiredTransfer)
			if err != nil {
				sourceUe.Log.Error("SendUpdateSmContextN2HandoverPreparing Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionIDUint8))
				continue
			}
			send.AppendPDUSessionResourceSetupListHOReq(&pduSessionReqList, pduSessionIDUint8, smContext.Snssai, n2Rsp)
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
		sourceUe.AmfUe.SetOnGoing(&amfContext.OnGoingProcedureWithPrio{
			Procedure: amfContext.OnGoingProcedureNothing,
		})
		err := sourceUe.Ran.NGAPSender.SendHandoverPreparationFailure(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, *cause, nil)
		if err != nil {
			sourceUe.Log.Error("error sending handover preparation failure", zap.Error(err))
			return
		}
		sourceUe.Log.Info("sent handover preparation failure to source UE")
		return
	}

	err := amfUe.UpdateNH()
	if err != nil {
		sourceUe.Log.Error("error updating NH", zap.Error(err))
		return
	}

	amfSelf := amfContext.AMFSelf()

	operatorInfo, err := amfSelf.GetOperatorInfo(ctx)
	if err != nil {
		sourceUe.Log.Error("Could not get operator info", zap.Error(err))
		return
	}

	targetUe, err := targetRan.NewRanUe(models.RanUeNgapIDUnspecified)
	if err != nil {
		logger.AmfLog.Error("error creating target ue", zap.Error(err))
		return
	}

	err = amfContext.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		logger.AmfLog.Error("attach source ue target ue error", zap.Error(err))
		return
	}

	err = targetUe.Ran.NGAPSender.SendHandoverRequest(
		ctx,
		targetUe.AmfUeNgapID,
		targetUe.HandOverType,
		targetUe.AmfUe.Ambr.Uplink,
		targetUe.AmfUe.Ambr.Downlink,
		targetUe.AmfUe.UESecurityCapability,
		targetUe.AmfUe.NCC,
		targetUe.AmfUe.NH,
		*cause,
		pduSessionReqList,
		*sourceToTargetTransparentContainer,
		operatorInfo.SupportedPLMN,
		operatorInfo.Guami,
	)
	if err != nil {
		sourceUe.Log.Error("error sending handover request to target UE", zap.Error(err))
		return
	}

	sourceUe.Log.Info("sent handover request to target UE")
}
