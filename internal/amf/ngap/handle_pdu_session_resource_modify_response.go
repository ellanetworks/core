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

// HandlePDUSessionResourceModifyResponse transfers each PDU session's Modify
// Response Transfer or Modify Unsuccessful Transfer to the SMF that owns the
// session (TS 38.413 §8.2.3.2).
func HandlePDUSessionResourceModifyResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.PDUSessionResourceModifyResponse) {
	// Both identities are mandatory but ignore criticality, so an absent one
	// still reaches the handler and leaves nothing to resolve by (§10.3.5).
	ueConn, ok := resolveUEIDs(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcPDUSessionResourceModify, ngap.TriggeringSuccessfulOutcome, ueAssociated(*msg.AMFUENGAPID, *msg.RANUENGAPID), msg.Diagnostics())

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	ueConn.TouchLastSeen()
	ueConn.Log(ctx).Debug("Handle PDUSessionResourceModifyResponse")

	ue := ueConn.UeContext()
	if ue == nil {
		return
	}

	for _, item := range msg.PDUSessionResourceModify {
		smContext, ok := ue.SmContextFindByPDUSessionID(uint8(item.PDUSessionID))
		if !ok {
			continue
		}

		if err := amfInstance.Session.UpdateSmContextN2InfoPduResModifyRsp(ctx, smContext.Ref, item.Transfer); err != nil {
			ueConn.Log(ctx).Warn("SMF did not take the PDU Session Resource Modify Response Transfer",
				logger.PDUSessionID(uint8(item.PDUSessionID)), zap.Error(err))
		}
	}

	for _, item := range msg.PDUSessionResourceFailed {
		ueConn.Log(ctx).Warn("NG-RAN node did not modify a PDU session",
			logger.PDUSessionID(uint8(item.PDUSessionID)))

		smContext, ok := ue.SmContextFindByPDUSessionID(uint8(item.PDUSessionID))
		if !ok {
			continue
		}

		if err := amfInstance.Session.UpdateSmContextN2InfoPduResModifyFail(ctx, smContext.Ref, item.Transfer); err != nil {
			ueConn.Log(ctx).Warn("SMF did not take the PDU Session Resource Modify Unsuccessful Transfer",
				logger.PDUSessionID(uint8(item.PDUSessionID)), zap.Error(err))
		}
	}
}
