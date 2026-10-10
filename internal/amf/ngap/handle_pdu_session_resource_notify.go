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

// HandlePDUSessionResourceNotify transfers each Notify Transfer to the SMF and
// deactivates each session the NG-RAN node released on its own initiative
// (TS 38.413 §8.2.4.2).
func HandlePDUSessionResourceNotify(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.PDUSessionResourceNotify) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcPDUSessionResourceNotify, ngap.TriggeringInitiatingMessage, ueAssociated(msg.AMFUENGAPID, msg.RANUENGAPID), msg.Diagnostics())

	ueConn.TouchLastSeen()
	ueConn.Log(ctx).Debug("Handle PDUSessionResourceNotify")

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		ueConn.Log(ctx).Error("amfUe is nil")
		return
	}

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	for _, item := range msg.PDUSessionResourceNotify {
		smContext, ok := amfUe.SmContextFindByPDUSessionID(uint8(item.PDUSessionID))
		if !ok {
			continue
		}

		if err := amfInstance.Session.UpdateSmContextN2InfoNotify(ctx, smContext.Ref, item.Transfer); err != nil {
			ueConn.Log(ctx).Warn("SMF did not take the PDU Session Resource Notify Transfer",
				logger.PDUSessionID(uint8(item.PDUSessionID)), zap.Error(err))
		}
	}

	for _, item := range msg.PDUSessionResourceReleased {
		pduSessionID := uint8(item.PDUSessionID)

		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			ueConn.Log(ctx).Error("SmContext not found", logger.PDUSessionID(pduSessionID))
			continue
		}

		err := amfInstance.Session.DeactivateSmContext(ctx, smContext.Ref, false)
		if err != nil {
			ueConn.Log(ctx).Error("DeactivateSmContext failed", zap.Error(err), logger.PDUSessionID(pduSessionID))
			continue
		}

		ueConn.SetN2SessionInactive(pduSessionID)

		ueConn.Log(ctx).Info("deactivated PDU session released by gNB", logger.PDUSessionID(pduSessionID))
	}
}
