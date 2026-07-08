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
	"go.uber.org/zap"
)

func handleAuthenticationResponse(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	// An AUTHENTICATION RESPONSE is valid only during the attach authentication
	// sub-phase; out of order, ignore it to avoid re-verifying a stale challenge.
	if ue.RegStep() != mme.RegStepAuthenticating {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Authentication Response outside the authentication sub-phase")

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	c := ue.Conn()
	c.StopNASGuard()

	resp, err := eps.ParseAuthenticationResponse(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Authentication Response", zap.Error(err))
		return nasreply.Handled()
	}

	if c.AuthVector == nil || subtle.ConstantTimeCompare(resp.RES, c.AuthVector.XRES) != 1 {
		logger.From(ctx, logger.MmeLog).Warn("authentication failed: RES mismatch")
		rejectAuthentication(m, ctx, ue)

		return nasreply.Handled()
	}

	ue.SetKASME(c.AuthVector.KASME)

	// With K_ASME held in the security context, drop the vector: this clears the
	// retained XRES/K_ASME/RAND key material and makes AuthVector==nil mean "no
	// challenge in flight". Reset the per-procedure resync budget alongside it.
	c.AuthVector = nil
	c.SetResyncTried(false)

	logger.From(ctx, logger.MmeLog).Info("authentication succeeded")
	startSecurityMode(m, ctx, ue)

	return nasreply.Handled()
}
