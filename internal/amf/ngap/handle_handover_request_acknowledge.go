package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverRequestAcknowledge(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.HandoverRequestAcknowledge) {
	if msg.AMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMF UE NGAP ID is nil")
		return
	}

	targetUe := amfInstance.FindRanUeByAmfUeNgapID(*msg.AMFUENGAPID)
	if targetUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("AmfUeNgapID", *msg.AMFUENGAPID))
		return
	}

	if msg.RANUENGAPID != nil {
		targetUe.RanUeNgapID = *msg.RANUENGAPID
	}

	targetUe.Radio = ran
	targetUe.TouchLastSeen()
	logger.WithTrace(ctx, ran.Log).Debug("Handle Handover Request Acknowledge", zap.Any("RanUeNgapID", targetUe.RanUeNgapID), zap.Any("AmfUeNgapID", targetUe.AmfUeNgapID))

	amfUe := targetUe.AmfUe()
	if amfUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("amfUe is nil")
		return
	}

	var (
		pduSessionResourceHandoverList  ngapType.PDUSessionResourceHandoverList
		pduSessionResourceToReleaseList ngapType.PDUSessionResourceToReleaseListHOCmd
	)

	// describe in 23.502 4.9.1.3.2 step11

	for _, item := range msg.AdmittedItems {
		pduSessionIDUint8, ok := validPDUSessionID(item.PDUSessionID.Value)
		if !ok {
			logger.WithTrace(ctx, targetUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
			continue
		}

		transfer := item.HandoverRequestAcknowledgeTransfer
		if smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionIDUint8); exist {
			n2Rsp, err := amfInstance.Smf.UpdateSmContextN2HandoverPrepared(ctx, smContext.Ref, transfer)
			if err != nil {
				logger.WithTrace(ctx, targetUe.Log).Error("Send HandoverRequestAcknowledgeTransfer error", zap.Error(err))
				continue
			}

			handoverItem := ngapType.PDUSessionResourceHandoverItem{}
			handoverItem.PDUSessionID = item.PDUSessionID
			handoverItem.HandoverCommandTransfer = n2Rsp
			pduSessionResourceHandoverList.List = append(pduSessionResourceHandoverList.List, handoverItem)
		}
	}

	for _, item := range msg.FailedToSetupItems {
		pduSessionIDUint8, ok := validPDUSessionID(item.PDUSessionID.Value)
		if !ok {
			logger.WithTrace(ctx, targetUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
			continue
		}

		transfer := item.HandoverResourceAllocationUnsuccessfulTransfer
		if smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionIDUint8); exist {
			_, err := amfInstance.Smf.UpdateSmContextN2HandoverPrepared(ctx, smContext.Ref, transfer)
			if err != nil {
				logger.WithTrace(ctx, targetUe.Log).Error("Send HandoverResourceAllocationUnsuccessfulTransfer error", zap.Error(err))
			}
		}
	}

	sourceUe := targetUe.SourceUe
	if sourceUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("handover between different Ue has not been implement yet")
		return
	}

	logger.WithTrace(ctx, ran.Log).Debug("handle handover request acknowledge", zap.Int64("sourceRanUeNgapID", sourceUe.RanUeNgapID), zap.Int64("sourceAmfUeNgapID", sourceUe.AmfUeNgapID),
		zap.Int64("targetRanUeNgapID", targetUe.RanUeNgapID), zap.Int64("targetAmfUeNgapID", targetUe.AmfUeNgapID))

	if len(pduSessionResourceHandoverList.List) == 0 {
		logger.WithTrace(ctx, targetUe.Log).Info("handle Handover Preparation Failure [HoFailure In Target5GC NgranNode Or TargetSystem]")

		cause := &ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
			},
		}

		if sourceAmfUe := sourceUe.AmfUe(); sourceAmfUe != nil {
			sourceAmfUe.SetOnGoing(amf.OnGoingProcedureNothing)
		}

		if sourceUe.Radio == nil {
			logger.WithTrace(ctx, ran.Log).Error("source UE radio is nil, cannot send handover preparation failure")
			return
		}

		err := sourceUe.Radio.NGAPSender.SendHandoverPreparationFailure(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, *cause, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending handover preparation failure", zap.Error(err))
		}

		logger.WithTrace(ctx, ran.Log).Info("sent handover preparation failure to source UE")

		return
	}

	err := sourceUe.Radio.NGAPSender.SendHandoverCommand(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, sourceUe.HandOverType, pduSessionResourceHandoverList, pduSessionResourceToReleaseList, msg.TargetToSourceTransparentContainer)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending handover command to source UE", zap.Error(err))
	}

	logger.WithTrace(ctx, ran.Log).Info("sent handover command to source UE")
}
