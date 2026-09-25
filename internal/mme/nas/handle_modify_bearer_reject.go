// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
)

// handleModifyBearerReject abandons the modification when the UE rejects it
// (TS 24.301 §6.4.2.4), leaving the stored config stale so the backstop retries.
// Cause #43 means the UE holds no such bearer, so the MME deactivates it locally.
func handleModifyBearerReject(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, rej *eps.ModifyEPSBearerContextReject) nasreply.Disposition {
	p := m.LookupPDN(ue, uint8(rej.EPSBearerIdentity))

	if p != nil {
		m.StopESMGuard(p)
		m.ConcludeBearerModification(ctx, ue, p, false)
	}

	ueConn.Log(ctx).Warn("UE rejected EPS bearer modification")

	if p != nil && rej.Cause == eps.ESMCauseInvalidEPSBearerIdentity {
		m.DeactivatePDN(ctx, ue, p)
	}

	return nasreply.Handled()
}
