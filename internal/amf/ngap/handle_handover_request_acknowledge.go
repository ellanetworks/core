// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// buildPDUSessionResourceToReleaseItemHOCmd builds the Handover Command to-release
// item for a non-admitted PDU session, relaying the target's reported failure cause
// when decodable, otherwise a generic one.
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

	targetUe := amfInstance.FindUEByAmfUeNgapID(ran, models.AmfUeNgapID(*msg.AMFUENGAPID))
	if targetUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context on this radio", zap.Int64("AmfUeNgapID", *msg.AMFUENGAPID))
		sendUnknownLocalUEError(ctx, ran, msg.AMFUENGAPID, msg.RANUENGAPID)

		return
	}

	if msg.RANUENGAPID != nil {
		amfInstance.UpdateUERanNgapID(targetUe, models.RanUeNgapID(*msg.RANUENGAPID))
	}

	targetUe.TouchLastSeen()
	logger.WithTrace(ctx, targetUe.Log).Debug("Handle Handover Request Acknowledge", zap.Any("RanUeNgapID", int64(targetUe.RanUeNgapID)), zap.Any("AmfUeNgapID", int64(targetUe.AmfUeNgapID)))

	amfUe := targetUe.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("amfUe is nil")
		return
	}

	sourceUe := amfInstance.HandoverSource(amfUe)
	if sourceUe == nil {
		logger.WithTrace(ctx, targetUe.Log).Error("handover between different Ue has not been implement yet")
		return
	}

	// A duplicate or out-of-order HANDOVER REQUEST ACKNOWLEDGE: the staleness check
	// precedes any per-session SMF side effect, since UpdateSmContextN2HandoverPrepared
	// rebinds the downlink tunnel (TS 38.413 §10.4).
	if !amfInstance.HandoverPreparing(amfUe) {
		logger.WithTrace(ctx, targetUe.Log).Warn("Handover Request Acknowledge for a handover past the preparing stage; dropping")
		return
	}

	var (
		pduSessionResourceHandoverList  ngapType.PDUSessionResourceHandoverList
		pduSessionResourceToReleaseList ngapType.PDUSessionResourceToReleaseListHOCmd
		admittedPDU                     = make(map[uint8]struct{})
	)

	for _, item := range msg.AdmittedItems {
		pduSessionIDUint8, ok := validPDUSessionID(item.PDUSessionID.Value)
		if !ok {
			logger.WithTrace(ctx, targetUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
			continue
		}

		transfer := item.HandoverRequestAcknowledgeTransfer
		if smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionIDUint8); exist {
			n2Rsp, err := amfInstance.Session.UpdateSmContextN2HandoverPrepared(ctx, smContext.Ref, transfer)
			if err != nil {
				logger.WithTrace(ctx, targetUe.Log).Error("Send HandoverRequestAcknowledgeTransfer error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionIDUint8))
				continue
			}

			handoverItem := ngapType.PDUSessionResourceHandoverItem{}
			handoverItem.PDUSessionID = item.PDUSessionID
			handoverItem.HandoverCommandTransfer = n2Rsp
			pduSessionResourceHandoverList.List = append(pduSessionResourceHandoverList.List, handoverItem)
			admittedPDU[pduSessionIDUint8] = struct{}{}
		}
	}

	// Sessions the target did not admit go in the to-release list so the source
	// frees them (TS 38.413); they stay on the source, so no SMF update.
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

	logger.WithTrace(ctx, targetUe.Log).Debug("handle handover request acknowledge", zap.Int64("sourceRanUeNgapID", int64(sourceUe.RanUeNgapID)), zap.Int64("sourceAmfUeNgapID", int64(sourceUe.AmfUeNgapID)),
		zap.Int64("targetRanUeNgapID", int64(targetUe.RanUeNgapID)), zap.Int64("targetAmfUeNgapID", int64(targetUe.AmfUeNgapID)))

	if len(pduSessionResourceHandoverList.List) == 0 {
		logger.WithTrace(ctx, targetUe.Log).Info("handle Handover Preparation Failure [HoFailure In Target5GC NgranNode Or TargetSystem]")

		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
			},
		}

		if sourceUeContext := sourceUe.UeContext(); sourceUeContext != nil {
			amfInstance.ClearHandover(sourceUeContext)
		}

		if sourceUe.Radio() == nil {
			logger.WithTrace(ctx, targetUe.Log).Error("source UE radio is nil, cannot send handover preparation failure")
		} else {
			sourceUe.SendHandoverPreparationFailure(ctx, cause, nil)
		}

		// The target acknowledged and so holds a reserved UE context, but no session
		// survived core-side preparation. Its resources are reclaimed only by a
		// CN-initiated UE Context Release (TS 38.413 §8.4.2).
		targetUe.ReleaseAction = amf.UeContextReleaseHandover
		targetUe.SendUEContextReleaseCommand(ctx, cause.Present, cause.RadioNetwork.Value)

		return
	}

	if !amfInstance.MarkHandoverPrepared(amfUe, admittedPDU) {
		logger.WithTrace(ctx, targetUe.Log).Warn("Handover Request Acknowledge: handover advanced concurrently; dropping")
		return
	}

	pkt, err := send.BuildHandoverCommand(int64(sourceUe.AmfUeNgapID), int64(sourceUe.RanUeNgapID), sourceUe.HandOverType, pduSessionResourceHandoverList, pduSessionResourceToReleaseList, msg.TargetToSourceTransparentContainer)
	if err != nil {
		logger.WithTrace(ctx, targetUe.Log).Error("error building handover command", zap.Error(err))
		return
	}

	sourceUe.SendNGAP(ctx, send.NGAPProcedureHandoverCommand, pkt)
}
