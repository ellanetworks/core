// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

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

func failedSessionCauses(failed ngap.PDUSessionResourceFailedToSetupListHOAck) map[ngap.PDUSessionID]ngap.Cause {
	causes := make(map[ngap.PDUSessionID]ngap.Cause, len(failed))

	for _, item := range failed {
		if received, err := ngap.ParseHandoverResourceAllocationUnsuccessfulTransfer(item.Transfer); err == nil {
			causes[item.PDUSessionID] = received.Cause
		}
	}

	return causes
}

// TS 38.413 §8.4.2.2
func reportFailedSessionsToSmf(ctx context.Context, amfInstance *amf.AMF, targetUe *amf.UeConn, amfUe *amf.UeContext, failed ngap.PDUSessionResourceFailedToSetupListHOAck, admitted map[uint8]struct{}) {
	for _, item := range failed {
		pduSessionID, ok := validPDUSessionID(int64(item.PDUSessionID))
		if !ok {
			logger.WithTrace(ctx, targetUe.Log()).Error("invalid PDU session ID in the failed-to-setup list", zap.Int64("pduSessionID", int64(item.PDUSessionID)))
			continue
		}

		if _, isAdmitted := admitted[pduSessionID]; isAdmitted {
			logger.WithTrace(ctx, targetUe.Log()).Warn("PDU session reported both admitted and failed to setup; keeping it handed over", zap.Uint8("pduSessionID", pduSessionID))
			continue
		}

		smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !exist {
			continue
		}

		if err := amfInstance.Session.UpdateSmContextN2HandoverFailed(ctx, smContext.Ref, item.Transfer); err != nil {
			logger.WithTrace(ctx, targetUe.Log()).Error("failed to hand the target's handover refusal to the SMF",
				zap.Error(err), zap.Uint8("pduSessionID", pduSessionID))
		}
	}
}

func releaseItems(ctx context.Context, targetUe *amf.UeConn, unadmitted []amf.HandoverCandidate, targetCauses map[ngap.PDUSessionID]ngap.Cause) ngap.PDUSessionResourceToReleaseListHOCmd {
	if len(unadmitted) == 0 {
		return nil
	}

	out := make(ngap.PDUSessionResourceToReleaseListHOCmd, 0, len(unadmitted))

	for _, c := range unadmitted {
		cause := causeHOFailureInTarget

		switch {
		case c.Cause != nil:
			cause = *c.Cause
		default:
			if reported, ok := targetCauses[c.PDUSessionID]; ok {
				cause = reported
			}
		}

		item, err := toReleaseItemHOCmd(c.PDUSessionID, cause)
		if err != nil {
			logger.WithTrace(ctx, targetUe.Log()).Error("failed to build PDU session to-release item", zap.Error(err), zap.Int64("pduSessionID", int64(c.PDUSessionID)))
			continue
		}

		out = append(out, item)
	}

	return out
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
	logger.WithTrace(ctx, targetUe.Log()).Debug("Handle Handover Request Acknowledge", zap.Uint32("ran-ue-id", uint32(targetUe.RanUeNgapID)), zap.Uint64("amf-ue-id", uint64(targetUe.AmfUeNgapID)))

	amfUe := targetUe.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, targetUe.Log()).Error("amfUe is nil")
		return
	}

	fromEPS := amfInstance.HandoverFromEPS(amfUe)

	sourceUe := amfInstance.HandoverSource(amfUe)
	if sourceUe == nil && !fromEPS {
		logger.WithTrace(ctx, targetUe.Log()).Error("handover between different Ue has not been implement yet")
		return
	}

	if amfInstance.HandoverTarget(amfUe) != targetUe {
		logger.WithTrace(ctx, targetUe.Log()).Warn("Handover Request Acknowledge from a radio that is not the prepared target; dropping")
		return
	}

	if !amfInstance.HandoverPreparing(amfUe) {
		logger.WithTrace(ctx, targetUe.Log()).Warn("Handover Request Acknowledge for a handover past the preparing stage; dropping")
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
			logger.WithTrace(ctx, targetUe.Log()).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", int64(item.PDUSessionID)))
			continue
		}

		smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !exist {
			continue
		}

		n2Rsp, err := amfInstance.Session.UpdateSmContextN2HandoverPrepared(ctx, smContext.Ref, item.Transfer)
		if err != nil {
			logger.WithTrace(ctx, targetUe.Log()).Error("Send HandoverRequestAcknowledgeTransfer error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			continue
		}

		admitted = append(admitted, ngap.PDUSessionResourceHandoverItem{
			PDUSessionID: item.PDUSessionID,
			Transfer:     ngap.TransferContainer(n2Rsp),
		})
		admittedPDU[pduSessionID] = struct{}{}
	}

	targetCauses := failedSessionCauses(msg.PDUSessionResourceFailedToSetup)

	reportFailedSessionsToSmf(ctx, amfInstance, targetUe, amfUe, msg.PDUSessionResourceFailedToSetup, admittedPDU)

	if len(admitted) == 0 {
		logger.WithTrace(ctx, targetUe.Log()).Info("handle Handover Preparation Failure [HoFailure In Target5GC NgranNode Or TargetSystem]")

		amfInstance.UnbindHandoverTarget(ctx, amfUe)

		if fromEPS {
			amfInstance.FailRelocationPreparation(amfUe,
				fmt.Errorf("%w: the target NG-RAN node admitted no PDU session", interworking.ErrTargetRefused))
		} else {
			amfInstance.ClearHandover(amfUe)

			if sourceUe.Radio() == nil {
				logger.WithTrace(ctx, targetUe.Log()).Error("source UE radio is nil, cannot send handover preparation failure")
			} else {
				sourceUe.SendHandoverPreparationFailure(ctx, causeHOFailureInTarget, nil, nil)
			}
		}

		targetUe.ReleaseAction = amf.UeContextReleaseHandover
		targetUe.SendUEContextReleaseCommand(ctx, causeHOFailureInTarget)

		return
	}

	unadmitted, ok := amfInstance.MarkHandoverPrepared(amfUe, admittedPDU)
	if !ok {
		logger.WithTrace(ctx, targetUe.Log()).Warn("Handover Request Acknowledge: handover advanced concurrently; dropping")
		return
	}

	if fromEPS {
		logger.WithTrace(ctx, targetUe.Log()).Info("Handover Request Acknowledge (EPS to 5GS)",
			zap.Int("admitted", len(admitted)), zap.Int("not-handed-over", len(unadmitted)))
		amfInstance.FinishRelocationPreparation(amfUe, msg.TargetToSourceTransparentContainer, unadmitted)

		return
	}

	logger.WithTrace(ctx, targetUe.Log()).Debug("handle handover request acknowledge", zap.Uint32("source-ran-ue-id", uint32(sourceUe.RanUeNgapID)), zap.Uint64("source-amf-ue-id", uint64(sourceUe.AmfUeNgapID)),
		zap.Uint32("target-ran-ue-id", uint32(targetUe.RanUeNgapID)), zap.Uint64("target-amf-ue-id", uint64(targetUe.AmfUeNgapID)))

	sourceUe.SendHandoverCommand(ctx, admitted, releaseItems(ctx, targetUe, unadmitted, targetCauses), msg.TargetToSourceTransparentContainer)
}
