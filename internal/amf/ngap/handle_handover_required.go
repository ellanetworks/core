package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverRequired(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.HandoverRequired) {
	sourceUe := ran.FindUEByRanUeNgapID(msg.RANUENGAPID)
	if sourceUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("Cannot find UE", zap.Int64("RAN_UE_NGAP_ID", msg.RANUENGAPID))

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
		logger.WithTrace(ctx, ran.Log).Error("AMF UE NGAP ID mismatch", zap.Int64("expected", sourceUe.AmfUeNgapID), zap.Int64("received", msg.AMFUENGAPID))

		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID,
			},
		}

		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ran.Log).Info("sent error indication for AMF UE NGAP ID mismatch")

		return
	}

	amfUe := sourceUe.AmfUe()
	if amfUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("Cannot find amfUE from sourceUE")
		return
	}

	sourceUe.TouchLastSeen()

	if msg.TargetID.Present != ngapType.TargetIDPresentTargetRANNodeID {
		logger.WithTrace(ctx, ran.Log).Error("targetID type is not supported", zap.Int("targetID", msg.TargetID.Present))
		return
	}

	amfUe.SetOnGoing(amf.OnGoingProcedureN2Handover)

	if !amfUe.SecurityContextIsValid() {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [Authentication Failure]")

		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentNas,
			Nas: &ngapType.CauseNas{
				Value: ngapType.CauseNasPresentAuthenticationFailure,
			},
		}

		sourceUe.AmfUe().SetOnGoing(amf.OnGoingProcedureNothing)

		err := sourceUe.SendHandoverPreparationFailure(ctx, failureCause, nil)
		if err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover preparation failure", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, sourceUe.Log).Info("sent handover preparation failure to source UE")

		return
	}

	targetRanNodeID := util.RanIDToModels(msg.TargetID.TargetRANNodeID.GlobalRANNodeID)

	targetRan, ok := amfInstance.FindRadioByRanID(targetRanNodeID)
	if !ok {
		// handover between different AMF
		logger.WithTrace(ctx, sourceUe.Log).Warn("Handover required : cannot find target Ran Node Id in this AMF. Handover between different AMF has not been implemented yet", zap.Any("targetRanNodeID", targetRanNodeID))
		return
		// Described in (23.502 4.9.1.3.2) step 3.Namf_Communication_CreateUEContext Request
	}

	// Handover in same AMF
	sourceUe.HandOverType.Value = msg.HandoverType.Value

	var pduSessionReqList ngapType.PDUSessionResourceSetupListHOReq

	for _, pDUSessionResourceHoItem := range msg.PDUSessionResourceItems {
		if pDUSessionResourceHoItem.PDUSessionID.Value < 1 || pDUSessionResourceHoItem.PDUSessionID.Value > 15 {
			logger.WithTrace(ctx, sourceUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", pDUSessionResourceHoItem.PDUSessionID.Value))
			continue
		}

		pduSessionIDUint8 := uint8(pDUSessionResourceHoItem.PDUSessionID.Value)
		if smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionIDUint8); exist {
			n2Rsp, err := amfInstance.Smf.UpdateSmContextN2HandoverPreparing(ctx, smContext.Ref, pDUSessionResourceHoItem.HandoverRequiredTransfer)
			if err != nil {
				logger.WithTrace(ctx, sourceUe.Log).Error("SendUpdateSmContextN2HandoverPreparing Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionIDUint8))
				continue
			}

			send.AppendPDUSessionResourceSetupListHOReq(&pduSessionReqList, pduSessionIDUint8, smContext.Snssai, n2Rsp)
		}
	}

	if len(pduSessionReqList.List) == 0 {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [HoFailure In Target5GC NgranNode Or TargetSystem]")

		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
			},
		}

		sourceUe.AmfUe().SetOnGoing(amf.OnGoingProcedureNothing)

		err := sourceUe.SendHandoverPreparationFailure(ctx, failureCause, nil)
		if err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover preparation failure", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, sourceUe.Log).Info("sent handover preparation failure to source UE")

		return
	}

	err := amfUe.UpdateNH()
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("error updating NH", zap.Error(err))
		return
	}

	operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("Could not get operator info", zap.Error(err))
		return
	}

	snssaiList, err := amfInstance.ListOperatorSnssai(ctx)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("Could not list operator SNSSAI", zap.Error(err))
		return
	}

	targetUe, err := amfInstance.NewRanUe(targetRan, models.RanUeNgapIDUnspecified)
	if err != nil {
		logger.WithTrace(ctx, logger.AmfLog).Error("error creating target ue", zap.Error(err))
		return
	}

	err = amf.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		logger.WithTrace(ctx, logger.AmfLog).Error("attach source ue target ue error", zap.Error(err))
		return
	}

	err = targetUe.SendHandoverRequest(
		ctx,
		sourceUe.HandOverType,
		amfUe.Ambr.Uplink,
		amfUe.Ambr.Downlink,
		amfUe.UESecurityCapability,
		amfUe.NCC,
		amfUe.NH,
		msg.Cause,
		pduSessionReqList,
		msg.SourceToTargetTransparentContainer,
		snssaiList,
		operatorInfo.Guami,
	)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover request to target UE", zap.Error(err))
		return
	}

	logger.WithTrace(ctx, sourceUe.Log).Info("sent handover request to target UE")
}
