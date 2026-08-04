// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// HandleInitialContextSetupResponse completes the UE context setup and hands
// each session outcome to the SMF (TS 38.413 §8.3.1).
func HandleInitialContextSetupResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.InitialContextSetupResponse) {
	// Both identities are mandatory but ignore criticality, so an absent one
	// still reaches the handler and leaves nothing to resolve by (§10.3.5).
	ueConn, ok := resolveUEIDs(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcInitialContextSetup, ngap.TriggeringSuccessfulOutcome, ueAssociated(*msg.AMFUENGAPID, *msg.RANUENGAPID), msg.Diagnostics())

	ueConn.TouchLastSeen()

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("amfUe is nil")
		return
	}

	if len(msg.PDUSessionResourceSetup) > 0 {
		logger.WithTrace(ctx, ueConn.Log).Debug("Send PDUSessionResourceSetupResponseTransfer to SMF")

		for _, item := range msg.PDUSessionResourceSetup {
			pduSessionID := uint8(item.PDUSessionID)
			transfer := []byte(item.Transfer)

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				logger.WithTrace(ctx, ueConn.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				continue
			}

			err := amfInstance.Session.UpdateSmContextN2InfoPduResSetupRsp(ctx, smContext.Ref, transfer)
			if err != nil {
				logger.WithTrace(ctx, ueConn.Log).Error("SendUpdateSmContextN2Info[PDUSessionResourceSetupResponseTransfer] Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			}
		}
	}

	if len(msg.PDUSessionResourceFailed) > 0 {
		logger.WithTrace(ctx, ueConn.Log).Debug("Send PDUSessionResourceSetupUnsuccessfulTransfer to SMF")

		for _, item := range msg.PDUSessionResourceFailed {
			pduSessionID := uint8(item.PDUSessionID)
			transfer := []byte(item.Transfer)

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				logger.WithTrace(ctx, ueConn.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				continue
			}

			err := amfInstance.Session.UpdateSmContextN2InfoPduResSetupFail(ctx, smContext.Ref, transfer)
			if err != nil {
				logger.WithTrace(ctx, ueConn.Log).Error("SendUpdateSmContextN2Info[PDUSessionResourceSetupUnsuccessfulTransfer] Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			}
		}
	}

	ueConn.MarkICSCompleted()

	// A UE returning to CM-CONNECTED applies any policy change deferred while it was
	// idle. Skipped mid-registration: the session was just established with the
	// current policy and the UE is not yet Registered.
	if amfUe.State() == amf.Registered {
		amfInstance.ReconcileSessionsForUE(ctx, amfUe)
	}
}
