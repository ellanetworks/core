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

// HandlePDUSessionResourceModifyResponse records the NG-RAN node's modify
// outcome (TS 38.413 §8.2.3).
//
// §8.2.3.2 also has the AMF transfer each Modify Response Transfer or Modify
// Unsuccessful Transfer to the SMF that owns the session. Ella Core's SMF has no
// entry point for either, so the outcome is logged and the session keeps the
// parameters the SMF applied when it built the request.
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
	logger.WithTrace(ctx, ueConn.Log).Debug("Handle PDUSessionResourceModifyResponse", zap.Uint64("amf-ue-id", uint64(ueConn.AmfUeNgapID)))

	// The transfer carries the NG-RAN node's cause but is opaque here, so the
	// rejection is surfaced by session id: without it a modification the RAN
	// refused is indistinguishable from one it applied.
	for _, item := range msg.PDUSessionResourceFailed {
		logger.WithTrace(ctx, ueConn.Log).Warn("NG-RAN node did not modify a PDU session",
			zap.Uint8("pdu-session-id", uint8(item.PDUSessionID)))
	}
}
