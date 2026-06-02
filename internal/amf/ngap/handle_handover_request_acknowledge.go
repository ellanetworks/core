package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// buildPDUSessionResourceToReleaseItemHOCmd builds the Handover Command
// to-release item for a PDU session the target did not admit. The failure cause
// the target reported (in the Handover Resource Allocation Unsuccessful
// Transfer) is relayed to the source when it can be decoded, otherwise a
// generic target-failure cause is used.
func buildPDUSessionResourceToReleaseItemHOCmd(pduSessionID ngapType.PDUSessionID, unsuccessful aper.OctetString) (ngapType.PDUSessionResourceToReleaseItemHOCmd, error) {
	cause := ngapType.Cause{
		Present: ngapType.CausePresentRadioNetwork,
		RadioNetwork: &ngapType.CauseRadioNetwork{
			Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
		},
	}

	var received ngapType.HandoverResourceAllocationUnsuccessfulTransfer
	if err := aper.UnmarshalWithParams(unsuccessful, &received, "valueExt"); err == nil {
		cause = received.Cause
	}

	transfer, err := aper.MarshalWithParams(ngapType.HandoverPreparationUnsuccessfulTransfer{Cause: cause}, "valueExt")
	if err != nil {
		return ngapType.PDUSessionResourceToReleaseItemHOCmd{}, err
	}

	return ngapType.PDUSessionResourceToReleaseItemHOCmd{
		PDUSessionID:                            pduSessionID,
		HandoverPreparationUnsuccessfulTransfer: transfer,
	}, nil
}

func HandleHandoverRequestAcknowledge(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.HandoverRequestAcknowledge) {
	if msg.AMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMF UE NGAP ID is nil")
		return
	}

	targetUe := ran.FindUEByAmfUeNgapID(*msg.AMFUENGAPID)
	if targetUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context on this radio", zap.Int64("AmfUeNgapID", *msg.AMFUENGAPID))
		sendUnknownLocalUEError(ctx, ran)

		return
	}

	if msg.RANUENGAPID != nil {
		ran.UpdateUERanNgapID(targetUe, *msg.RANUENGAPID)
	}

	targetUe.TouchLastSeen()
	logger.WithTrace(ctx, targetUe.Log).Debug("Handle Handover Request Acknowledge", zap.Any("RanUeNgapID", targetUe.RanUeNgapID), zap.Any("AmfUeNgapID", targetUe.AmfUeNgapID))

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
				logger.WithTrace(ctx, targetUe.Log).Error("Send HandoverRequestAcknowledgeTransfer error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionIDUint8))
				continue
			}

			handoverItem := ngapType.PDUSessionResourceHandoverItem{}
			handoverItem.PDUSessionID = item.PDUSessionID
			handoverItem.HandoverCommandTransfer = n2Rsp
			pduSessionResourceHandoverList.List = append(pduSessionResourceHandoverList.List, handoverItem)
		}
	}

	// PDU sessions the target could not admit are listed for release so the
	// source NG-RAN frees their resources (TS 38.413 §8.4.1.2). They are not
	// switched to the target, so their SMF context is left on the source path.
	for _, item := range msg.FailedToSetupItems {
		if _, ok := validPDUSessionID(item.PDUSessionID.Value); !ok {
			logger.WithTrace(ctx, targetUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
			continue
		}

		releaseItem, err := buildPDUSessionResourceToReleaseItemHOCmd(item.PDUSessionID, item.HandoverResourceAllocationUnsuccessfulTransfer)
		if err != nil {
			logger.WithTrace(ctx, targetUe.Log).Error("failed to build PDU session to-release item", zap.Error(err), zap.Int64("pduSessionID", item.PDUSessionID.Value))
			continue
		}

		pduSessionResourceToReleaseList.List = append(pduSessionResourceToReleaseList.List, releaseItem)
	}

	sourceUe := targetUe.SourceUe
	if sourceUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("handover between different Ue has not been implement yet")
		return
	}

	logger.WithTrace(ctx, targetUe.Log).Debug("handle handover request acknowledge", zap.Int64("sourceRanUeNgapID", sourceUe.RanUeNgapID), zap.Int64("sourceAmfUeNgapID", sourceUe.AmfUeNgapID),
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
			sourceAmfUe.NasConn().Procedures.End(procedure.N2Handover)
		}

		if sourceUe.Radio() == nil {
			logger.WithTrace(ctx, targetUe.Log).Error("source UE radio is nil, cannot send handover preparation failure")
			return
		}

		err := sourceUe.Radio().NGAPSender.SendHandoverPreparationFailure(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, *cause, nil)
		if err != nil {
			logger.WithTrace(ctx, targetUe.Log).Error("error sending handover preparation failure", zap.Error(err))
		}

		return
	}

	err := sourceUe.Radio().NGAPSender.SendHandoverCommand(ctx, sourceUe.AmfUeNgapID, sourceUe.RanUeNgapID, sourceUe.HandOverType, pduSessionResourceHandoverList, pduSessionResourceToReleaseList, msg.TargetToSourceTransparentContainer)
	if err != nil {
		logger.WithTrace(ctx, targetUe.Log).Error("error sending handover command to source UE", zap.Error(err))
	}
}
