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

// HandlePDUSessionResourceReleaseResponse hands each release outcome to the SMF
// (TS 38.413 §8.2.2).
func HandlePDUSessionResourceReleaseResponse(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.PDUSessionResourceReleaseResponse) {
	// Both identities are mandatory but ignore criticality, so an absent one
	// still reaches the handler and leaves nothing to resolve by (§10.3.5).
	// resolveUEIDs also rejects a response naming a UE on another radio.
	ueConn, ok := resolveUEIDs(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcPDUSessionResourceRelease, ngap.TriggeringSuccessfulOutcome, ueAssociated(*msg.AMFUENGAPID, *msg.RANUENGAPID), msg.Diagnostics())

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	ueConn.TouchLastSeen()

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("amfUe is nil")
		return
	}

	if len(msg.PDUSessionResourceReleased) > 0 {
		logger.WithTrace(ctx, ueConn.Log).Debug("Send PDUSessionResourceReleaseResponseTransfer to SMF")

		for _, item := range msg.PDUSessionResourceReleased {
			pduSessionID := uint8(item.PDUSessionID)

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				logger.WithTrace(ctx, ueConn.Log).Warn("SmContext not found during release response (may already be removed by SMF)",
					zap.Uint8("PduSessionID", pduSessionID))
			}

			if smContext != nil {
				// The SMF discards the context only once the UE's RELEASE COMPLETE
				// has landed as well, which may be either side of this response
				// (TS 23.502 §4.3.4.2). Drop the AMF's reference on whichever leg
				// it reports gone; until then the session is merely inactive.
				removed, err := amfInstance.Session.UpdateSmContextN2InfoPduResRelRsp(ctx, smContext.Ref)
				if err != nil {
					logger.WithTrace(ctx, ueConn.Log).Error("SendUpdateSmContextN2InfoPduResRelRsp failed", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
				}

				if removed {
					amfUe.DeleteSmContext(pduSessionID)

					continue
				}
			}

			amfUe.SetSmContextInactive(pduSessionID)
		}
	}
}
