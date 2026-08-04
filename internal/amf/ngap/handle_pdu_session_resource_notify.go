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
// the NG-RAN node released on its own initiative (TS 38.413 §8.2.6).
func HandlePDUSessionResourceNotify(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.PDUSessionResourceNotify) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcPDUSessionResourceNotify, ngap.TriggeringInitiatingMessage, ueAssociated(msg.AMFUENGAPID, msg.RANUENGAPID), msg.Diagnostics())

	ueConn.TouchLastSeen()
	logger.WithTrace(ctx, ueConn.Log).Debug("Handle PDUSessionResourceNotify", zap.Uint64("amf-ue-id", uint64(ueConn.AmfUeNgapID)))

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("amfUe is nil")
		return
	}

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	if len(msg.PDUSessionResourceNotify) > 0 {
		logger.WithTrace(ctx, ueConn.Log).Warn("PDUSessionResourceNotifyList received but QoS flow notification forwarding is not implemented")
	}

	for _, item := range msg.PDUSessionResourceReleased {
		pduSessionID := uint8(item.PDUSessionID)

		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			logger.WithTrace(ctx, ueConn.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
			continue
		}

		err := amfInstance.Session.DeactivateSmContext(ctx, smContext.Ref)
		if err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("DeactivateSmContext failed", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			continue
		}

		amfUe.SetSmContextInactive(pduSessionID)

		logger.WithTrace(ctx, ueConn.Log).Info("deactivated PDU session released by gNB", zap.Uint8("PduSessionID", pduSessionID))
	}
}
