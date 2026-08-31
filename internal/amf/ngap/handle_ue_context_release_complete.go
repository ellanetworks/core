// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/ngap"
)

// HandleUEContextReleaseComplete completes the release: the UE-associated
// logical NG-connection is torn down and its identities freed (TS 38.413 §8.3).
func HandleUEContextReleaseComplete(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.UEContextReleaseComplete) {
	// Both identities are mandatory but ignore criticality, so an absent one
	// still reaches the handler and leaves nothing to resolve by (§10.3.5).
	ueConn, ok := resolveUEIDs(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcUEContextRelease, ngap.TriggeringSuccessfulOutcome, ueAssociated(*msg.AMFUENGAPID, *msg.RANUENGAPID), msg.Diagnostics())

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	// Cancel the release-supervision guard so it does not also run the cleanup.
	ueConn.StopReleaseGuard()

	amfInstance.ReleaseUeConn(ctx, ueConn)
}
