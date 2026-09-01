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

// HandlePDUSessionResourceNotify records a QoS-flow status change or a session
// the NG-RAN node released on its own initiative (TS 38.413 §8.2.4).
//
// §8.2.4.2 has the AMF transfer each Notify Transfer or Notify Released Transfer
// to the SMF that owns the session. Ella Core's SMF acts on the released list
// alone; it has no entry point for a QoS-flow notification, so a GBR flow the
// NG-RAN node reports as no longer fulfilled is logged and not acted on.
func HandlePDUSessionResourceNotify(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.PDUSessionResourceNotify) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcPDUSessionResourceNotify, ngap.TriggeringInitiatingMessage, ueAssociated(msg.AMFUENGAPID, msg.RANUENGAPID), msg.Diagnostics())

	ueConn.TouchLastSeen()
	logger.WithTrace(ctx, ueConn.Log()).Debug("Handle PDUSessionResourceNotify", zap.Uint64("amf-ue-id", uint64(ueConn.AmfUeNgapID)))

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ueConn.Log()).Error("amfUe is nil")
		return
	}

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	for _, item := range msg.PDUSessionResourceNotify {
		logger.WithTrace(ctx, ueConn.Log()).Warn("QoS flow status change not forwarded to the SMF (TS 38.413 §8.2.4.2)",
			zap.Uint8("pdu-session-id", uint8(item.PDUSessionID)))
	}

	for _, item := range msg.PDUSessionResourceReleased {
		pduSessionID := uint8(item.PDUSessionID)

		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			logger.WithTrace(ctx, ueConn.Log()).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
			continue
		}

		err := amfInstance.Session.DeactivateSmContext(ctx, smContext.Ref)
		if err != nil {
			logger.WithTrace(ctx, ueConn.Log()).Error("DeactivateSmContext failed", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			continue
		}

		ueConn.SetN2SessionInactive(pduSessionID)

		logger.WithTrace(ctx, ueConn.Log()).Info("deactivated PDU session released by gNB", zap.Uint8("PduSessionID", pduSessionID))
	}
}
