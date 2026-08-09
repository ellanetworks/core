// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// toReleaseItemHOCmd builds the Handover Command to-release item for a session
// the target did not admit, relaying the cause the target reported when the
// transfer decodes and a generic one when it does not.
func toReleaseItemHOCmd(pduSessionID ngap.PDUSessionID, unsuccessful ngap.TransferContainer) (ngap.PDUSessionResourceToReleaseItemHOCmd, error) {
	cause := causeHoFailureInTarget

	if received, err := ngap.ParseHandoverResourceAllocationUnsuccessfulTransfer(unsuccessful); err == nil {
		cause = received.Cause
	}

	transfer, err := (&ngap.HandoverPreparationUnsuccessfulTransfer{Cause: cause}).Marshal()
	if err != nil {
		return ngap.PDUSessionResourceToReleaseItemHOCmd{}, err
	}

	return ngap.PDUSessionResourceToReleaseItemHOCmd{
		PDUSessionID: pduSessionID,
		Transfer:     transfer,
	}, nil
}

func HandleHandoverRequestAcknowledge(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.HandoverRequestAcknowledge) {
	if msg.AMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMF UE NGAP ID is nil")
		return
	}

	targetUe := amfInstance.FindUEByAmfUeNgapID(ran, models.AmfUeNgapID(*msg.AMFUENGAPID))
	if targetUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context on this radio", zap.Uint64("amf-ue-id", uint64(*msg.AMFUENGAPID)))
		sendErrorIndication(ctx, ran, msg.AMFUENGAPID, msg.RANUENGAPID, causeUnknownLocalUEID)

		return
	}

	targetUe.TouchLastSeen()
	logger.WithTrace(ctx, targetUe.Log).Debug("Handle Handover Request Acknowledge", zap.Uint32("ran-ue-id", uint32(targetUe.RanUeNgapID)), zap.Uint64("amf-ue-id", uint64(targetUe.AmfUeNgapID)))

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

	if amfInstance.HandoverTarget(amfUe) != targetUe {
		logger.WithTrace(ctx, targetUe.Log).Warn("Handover Request Acknowledge from a radio that is not the prepared target; dropping")
		return
	}

	// A duplicate or out-of-order HANDOVER REQUEST ACKNOWLEDGE: the staleness check
	// precedes any per-session SMF side effect, since UpdateSmContextN2HandoverPrepared
	// rebinds the downlink tunnel (TS 38.413 §10.4).
	if !amfInstance.HandoverPreparing(amfUe) {
		logger.WithTrace(ctx, targetUe.Log).Warn("Handover Request Acknowledge for a handover past the preparing stage; dropping")
		return
	}

	if msg.RANUENGAPID != nil {
		amfInstance.UpdateUERanNgapID(targetUe, models.RanUeNgapID(*msg.RANUENGAPID))
	}

	var (
		admitted    ngap.PDUSessionResourceHandoverList
		toRelease   ngap.PDUSessionResourceToReleaseListHOCmd
		admittedPDU = make(map[uint8]struct{})
	)

	for _, item := range msg.PDUSessionResourceAdmittedList {
		pduSessionID, ok := validPDUSessionID(int64(item.PDUSessionID))
		if !ok {
			logger.WithTrace(ctx, targetUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", int64(item.PDUSessionID)))
			continue
		}

		smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !exist {
			continue
		}

		n2Rsp, err := amfInstance.Session.UpdateSmContextN2HandoverPrepared(ctx, smContext.Ref, item.Transfer)
		if err != nil {
			logger.WithTrace(ctx, targetUe.Log).Error("Send HandoverRequestAcknowledgeTransfer error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			continue
		}

		admitted = append(admitted, ngap.PDUSessionResourceHandoverItem{
			PDUSessionID: item.PDUSessionID,
			Transfer:     ngap.TransferContainer(n2Rsp),
		})
		admittedPDU[pduSessionID] = struct{}{}
	}

	// Sessions the target did not admit go in the to-release list so the source
	// frees them (TS 38.413); they stay on the source, so no SMF update.
	for _, item := range msg.PDUSessionResourceFailedToSetup {
		if _, ok := validPDUSessionID(int64(item.PDUSessionID)); !ok {
			logger.WithTrace(ctx, targetUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", int64(item.PDUSessionID)))
			continue
		}

		releaseItem, err := toReleaseItemHOCmd(item.PDUSessionID, item.Transfer)
		if err != nil {
			logger.WithTrace(ctx, targetUe.Log).Error("failed to build PDU session to-release item", zap.Error(err), zap.Int64("pduSessionID", int64(item.PDUSessionID)))
			continue
		}

		toRelease = append(toRelease, releaseItem)
	}

	logger.WithTrace(ctx, targetUe.Log).Debug("handle handover request acknowledge", zap.Uint32("source-ran-ue-id", uint32(sourceUe.RanUeNgapID)), zap.Uint64("source-amf-ue-id", uint64(sourceUe.AmfUeNgapID)),
		zap.Uint32("target-ran-ue-id", uint32(targetUe.RanUeNgapID)), zap.Uint64("target-amf-ue-id", uint64(targetUe.AmfUeNgapID)))

	if len(admitted) == 0 {
		logger.WithTrace(ctx, targetUe.Log).Info("handle Handover Preparation Failure [HoFailure In Target5GC NgranNode Or TargetSystem]")

		if sourceUeContext := sourceUe.UeContext(); sourceUeContext != nil {
			amfInstance.ClearHandover(sourceUeContext)
			// The bind happens per session before this count is known, so a
			// preparation that admits nothing still leaves one behind.
			amfInstance.UnbindHandoverTarget(ctx, sourceUeContext)
		}

		if sourceUe.Radio() == nil {
			logger.WithTrace(ctx, targetUe.Log).Error("source UE radio is nil, cannot send handover preparation failure")
		} else {
			sourceUe.SendHandoverPreparationFailure(ctx, causeHoFailureInTarget, nil, nil)
		}

		// The target acknowledged and so holds a reserved UE context, but no session
		// survived core-side preparation. Its resources are reclaimed only by a
		// CN-initiated UE Context Release (TS 38.413 §8.4.2).
		targetUe.ReleaseAction = amf.UeContextReleaseHandover
		targetUe.SendUEContextReleaseCommand(ctx, causeHoFailureInTarget)

		return
	}

	if !amfInstance.MarkHandoverPrepared(amfUe, admittedPDU) {
		logger.WithTrace(ctx, targetUe.Log).Warn("Handover Request Acknowledge: handover advanced concurrently; dropping")
		return
	}

	sourceUe.SendHandoverCommand(ctx, admitted, toRelease, msg.TargetToSourceTransparentContainer)
}
