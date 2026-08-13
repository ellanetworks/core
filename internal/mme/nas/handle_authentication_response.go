// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"crypto/subtle"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
)

func handleAuthenticationResponse(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, resp *eps.AuthenticationResponse) nasreply.Disposition {
	// An AUTHENTICATION RESPONSE is valid only during the attach authentication
	// sub-phase; out of order, ignore it to avoid re-verifying a stale challenge.
	if ue.RegStep() != mme.RegStepAuthenticating {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Authentication Response outside the authentication sub-phase")

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	c := ueConn
	c.StopNASGuard()

	if c.AuthVector == nil || subtle.ConstantTimeCompare(resp.RES, c.AuthVector.XRES) != 1 {
		logger.From(ctx, logger.MmeLog).Warn("authentication failed: RES mismatch")
		rejectAuthentication(ctx, m, ue, ueConn)

		return nasreply.Handled()
	}

	ue.SetKASME(c.AuthVector.KASME)

	// With K_ASME held in the security context, drop the vector: this clears the
	// retained XRES/K_ASME/RAND key material and makes AuthVector==nil mean "no
	// challenge in flight". Reset the per-procedure resync budget alongside it.
	c.AuthVector = nil
	c.SetResyncTried(false)

	logger.From(ctx, logger.MmeLog).Info("authentication succeeded")

	if startSecurityMode(ctx, m, ue, ueConn, freshKeys) == securityModeNoCommonAlgorithm {
		rejectAttach(ctx, m, ue, ueConn, eps.EMMCauseUESecurityCapabilitiesMismatch)
	}

	return nasreply.Handled()
}
