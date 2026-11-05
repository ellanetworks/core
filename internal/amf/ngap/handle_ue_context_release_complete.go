// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// HandleUEContextReleaseComplete completes the release: the UE-associated
// logical NG-connection is torn down and its identities freed (TS 38.413 §8.3).
func HandleUEContextReleaseComplete(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.UEContextReleaseComplete) {
	// Both identities are mandatory but ignore criticality, so an absent one
	// still reaches the handler and leaves nothing to resolve by (§10.3.5).
	if msg.AMFUENGAPID == nil || msg.RANUENGAPID == nil {
		ran.Log(ctx).Warn("UE-associated NGAP message without both UE NGAP IDs")

		return
	}

	amfID, ranID := models.AmfUeNgapID(*msg.AMFUENGAPID), models.RanUeNgapID(*msg.RANUENGAPID)

	ueConn := amfInstance.FindUEByAmfUeNgapID(ran, amfID)
	if ueConn == nil {
		ran.Log(ctx).Info("UE Context Release Complete for a connection the AMF no longer holds",
			zap.Uint64("amf_ue_ngap_id", uint64(amfID)), logger.RanUeNgapID(ranID))

		return
	}

	if ueConn.RanUeNgapID() != ranID {
		ran.Log(ctx).Warn("UE Context Release Complete with an inconsistent RAN-UE-NGAP-ID; releasing locally",
			zap.Uint64("amf_ue_ngap_id", uint64(amfID)),
			zap.Uint32("stored_ran_ue_ngap_id", uint32(ueConn.RanUeNgapID())),
			zap.Uint32("received_ran_ue_ngap_id", uint32(ranID)))
		amfInstance.ReleaseConnsWithRanUeNgapID(ctx, ran, ranID)

		return
	}

	attrs.IdentifyUE(ctx, ueConn.UeContext().Supi().String())

	if !ueConn.ClaimReleaseCompletion() {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcUEContextRelease, ngap.TriggeringSuccessfulOutcome, ueAssociated(*msg.AMFUENGAPID, *msg.RANUENGAPID), msg.Diagnostics())

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	var served []uint8
	if msg.PDUSessionResourceList != nil {
		served = make([]uint8, 0, len(msg.PDUSessionResourceList))
		for _, item := range msg.PDUSessionResourceList {
			served = append(served, uint8(item.PDUSessionID))
		}
	}

	if amfInstance.ReleaseUeConnServedBy(ctx, ueConn, served) {
		ueConn.Log(ctx).Info("UE idle", logger.RAT(metrics.RAT5G))
	}
}
