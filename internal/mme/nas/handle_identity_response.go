// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func handleIdentityResponse(ctx context.Context, m *mme.MME, ue *mme.UeContext, resp *eps.IdentityResponse) nasreply.Disposition {
	// An IDENTITY RESPONSE is valid only during the attach authentication sub-phase
	// (admissible without integrity, TS 24.301 §4.4.4.3); out of order it must not
	// re-set the IMSI or restart authentication.
	if ue.RegStep() != mme.RegStepAuthenticating {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Identity Response outside the authentication sub-phase")

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ue.Conn().StopNASGuard()

	if resp.MobileIdentity.IMSI == nil {
		logger.From(ctx, logger.MmeLog).Warn("Identity Response carries no IMSI",
			zap.Stringer("identity", resp.MobileIdentity))

		return nasreply.Handled()
	}

	m.SetIMSI(ue, string(*resp.MobileIdentity.IMSI))
	authenticateOrReject(ctx, m, ue)

	return nasreply.Handled()
}
