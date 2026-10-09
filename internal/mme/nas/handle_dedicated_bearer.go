// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
)

func handleActivateDedicatedBearerAccept(ctx context.Context, m *mme.MME, ue *mme.UeContext, accept *eps.ActivateDedicatedEPSBearerContextAccept) nasreply.Disposition {
	if !m.DedicatedBearerAccepted(ctx, ue, uint8(accept.EPSBearerIdentity)) {
		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	return nasreply.Handled()
}

func handleActivateDedicatedBearerReject(ctx context.Context, m *mme.MME, ue *mme.UeContext, reject *eps.ActivateDedicatedEPSBearerContextReject) nasreply.Disposition {
	ebi := uint8(reject.EPSBearerIdentity)

	if !m.DedicatedActivating(ue, ebi) {
		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	m.FailDedicatedBearer(ctx, ue, ebi, "the UE rejected it: "+reject.Cause.String())

	return nasreply.Handled()
}
