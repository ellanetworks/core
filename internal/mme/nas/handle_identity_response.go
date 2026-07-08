// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func handleIdentityResponse(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	// An IDENTITY RESPONSE is valid only during the attach authentication sub-phase
	// (admissible without integrity, TS 24.301 §4.4.4.3); out of order it must not
	// re-set the IMSI or restart authentication.
	if ue.RegStep() != mme.RegStepAuthenticating {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Identity Response outside the authentication sub-phase")

		return
	}

	ue.Conn().StopNASGuard()

	resp, err := eps.ParseIdentityResponse(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Identity Response", zap.Error(err))
		return
	}

	m.SetIMSI(ue, mobileIdentityDigits(resp.MobileIdentity))
	authenticateOrReject(m, ctx, ue)
}
