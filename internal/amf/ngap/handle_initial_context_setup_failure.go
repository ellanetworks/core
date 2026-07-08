// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandleInitialContextSetupFailure(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.InitialContextSetupFailure) {
	logger.WithTrace(ctx, ran.Log).Warn("Initial Context Setup Failure received", logger.Cause(causeToString(msg.Cause)))

	ueConn, ok := resolveUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

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

	if msg.PDUSessionResourceFailedToSetupItems == nil {
		return
	}

	logger.WithTrace(ctx, ueConn.Log).Debug("Send PDUSessionResourceSetupUnsuccessfulTransfer to SMF")

	for _, item := range msg.PDUSessionResourceFailedToSetupItems {
		pduSessionID, ok := validPDUSessionID(item.PDUSessionID.Value)
		if !ok {
			logger.WithTrace(ctx, ueConn.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
			continue
		}

		transfer := item.PDUSessionResourceSetupUnsuccessfulTransfer

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
