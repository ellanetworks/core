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

// toReleaseItemHOCmd builds the Handover Command to-release item naming a session
// that could not be handed over, and the cause to report for it
// (TS 38.413 §9.2.3.2).
func toReleaseItemHOCmd(pduSessionID ngap.PDUSessionID, cause ngap.Cause) (ngap.PDUSessionResourceToReleaseItemHOCmd, error) {
	transfer, err := (&ngap.HandoverPreparationUnsuccessfulTransfer{Cause: cause}).Marshal()
	if err != nil {
		return ngap.PDUSessionResourceToReleaseItemHOCmd{}, err
	}

	return ngap.PDUSessionResourceToReleaseItemHOCmd{
		PDUSessionID: pduSessionID,
		Transfer:     transfer,
	}, nil
}

// reportFailedToSetup hands each session the target refused to its SMF and
// collects the causes the target gave.
//
// TS 38.413 §8.4.2.2: "Upon reception of the HANDOVER REQUEST ACKNOWLEDGE message
// the AMF shall, for each PDU session indicated in the PDU Session ID IE, transfer
// transparently the Handover Request Acknowledge Transfer IE or Handover Resource
// Allocation Unsuccessful Transfer IE to the SMF associated with the concerned PDU
// session." The unsuccessful half is what lets the SMF free whatever preparation
// reserved for a session the target then refused (TS 23.502 §4.9.1.3.2 step 11a).
// The user plane is not touched here: the UE is still on the source until it
// arrives at the target, and a handover cancelled in between must leave it intact.
func reportFailedToSetup(ctx context.Context, amfInstance *amf.AMF, targetUe *amf.UeConn, amfUe *amf.UeContext, failed ngap.PDUSessionResourceFailedToSetupListHOAck) map[ngap.PDUSessionID]ngap.Cause {
	causes := make(map[ngap.PDUSessionID]ngap.Cause, len(failed))

	for _, item := range failed {
		if received, err := ngap.ParseHandoverResourceAllocationUnsuccessfulTransfer(item.Transfer); err == nil {
			causes[item.PDUSessionID] = received.Cause
		}

		pduSessionID, ok := validPDUSessionID(int64(item.PDUSessionID))
		if !ok {
			logger.WithTrace(ctx, targetUe.Log).Error("invalid PDU session ID in the failed-to-setup list", zap.Int64("pduSessionID", int64(item.PDUSessionID)))
			continue
		}

		smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !exist {
			continue
		}

		// A relay: a refusal by the SMF to take the news does not change the
		// handover, which the target has already answered.
		if err := amfInstance.Session.UpdateSmContextN2HandoverFailed(ctx, smContext.Ref, item.Transfer); err != nil {
			logger.WithTrace(ctx, targetUe.Log).Error("failed to hand the target's handover refusal to the SMF",
				zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
		}
	}

	return causes
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

	targetCauses := reportFailedToSetup(ctx, amfInstance, targetUe, amfUe, msg.PDUSessionResourceFailedToSetup)

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

	unadmitted, ok := amfInstance.MarkHandoverPrepared(amfUe, admittedPDU)
	if !ok {
		logger.WithTrace(ctx, targetUe.Log).Warn("Handover Request Acknowledge: handover advanced concurrently; dropping")
		return
	}

	// Every candidate the target did not admit — whether it refused it, answered for
	// it in neither list, or the 5GC never offered it — is reported to the source so
	// it releases the session (TS 38.413 §8.4.1.2). The set is disjoint from the
	// admitted list by construction.
	var toRelease ngap.PDUSessionResourceToReleaseListHOCmd

	for _, c := range unadmitted {
		// The 5GC's own reason where it could not offer the session, the target's
		// where it refused one, and a generic one where the target answered for it
		// in neither list.
		cause := causeHoFailureInTarget

		if reported, ok := targetCauses[c.PDUSessionID]; ok {
			cause = reported
		}

		if c.Cause != nil {
			cause = *c.Cause
		}

		releaseItem, err := toReleaseItemHOCmd(c.PDUSessionID, cause)
		if err != nil {
			logger.WithTrace(ctx, targetUe.Log).Error("failed to build PDU session to-release item", zap.Error(err), zap.Int64("pduSessionID", int64(c.PDUSessionID)))
			continue
		}

		toRelease = append(toRelease, releaseItem)
	}

	sourceUe.SendHandoverCommand(ctx, admitted, toRelease, msg.TargetToSourceTransparentContainer)
}
