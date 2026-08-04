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

// HandleInitialContextSetupFailure abandons the UE context setup and hands each
// failed session to the SMF (TS 38.413 §8.3.1.4).
func HandleInitialContextSetupFailure(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.InitialContextSetupFailure) {
	// The Cause is mandatory but ignore criticality, so it may be absent.
	cause := "absent"
	if msg.Cause != nil {
		cause = msg.Cause.String()
	}

	logger.WithTrace(ctx, ran.Log).Warn("Initial Context Setup Failure received", logger.Cause(cause))

	ueConn, ok := resolveUEIDs(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcInitialContextSetup, ngap.TriggeringUnsuccessfulOutcome, ueAssociated(*msg.AMFUENGAPID, *msg.RANUENGAPID), msg.Diagnostics())

	ueConn.TouchLastSeen()

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("amfUe is nil")
		return
	}

	if conn := amfUe.Conn(); conn != nil && conn.NASGuardActive() {
		conn.StopNASGuard()

		amfUe.Deregister(ctx)
		amfUe.ClearRegistrationRequestData()
	}

	if msg.PDUSessionResourceFailed == nil {
		return
	}

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
