// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func handleAuthenticationFailure(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	// An AUTHENTICATION FAILURE is admissible without integrity protection
	// (TS 24.301 §4.4.4.3) and can be injected. It is valid only during the attach
	// authentication sub-phase; in any other state its handling is
	// implementation-dependent (§7.4) — drop it, never release the UE.
	if ue.RegStep() != mme.RegStepAuthenticating {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Authentication Failure outside the authentication sub-phase")

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	// No challenge is in flight during the identity sub-window, so an
	// AUTHENTICATION FAILURE has nothing to fail.
	c := ue.Conn()
	if c.AuthVector == nil {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Authentication Failure with no authentication in progress")

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	resp, err := eps.ParseAuthenticationFailure(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Authentication Failure", zap.Error(err))
		return nasreply.Handled()
	}

	logger.From(ctx, logger.MmeLog).Info("Authentication Failure", zap.Uint8("emm-cause", resp.Cause))

	// A cause outside the enumeration (#20, #21, #26) is semantically incorrect:
	// ignore it and leave the procedure and its guard (T3460) running; the UE is
	// not released (TS 24.301 §7.8). Stop the guard only for an enumerated cause.
	switch resp.Cause {
	case mme.EmmCauseMACFailure, mme.EmmCauseSynchFailure, mme.EmmCauseNonEPSAuthUnacceptable:
	default:
		logger.From(ctx, logger.MmeLog).Warn("ignoring Authentication Failure with an out-of-enumeration cause",
			zap.Uint8("emm-cause", resp.Cause))

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	c.StopNASGuard()

	if resp.Cause == mme.EmmCauseSynchFailure && len(resp.AUTS) > 0 && !c.ResyncTried() && c.AuthVector != nil {
		c.SetResyncTried(true)

		logger.From(ctx, logger.MmeLog).Info("re-synchronising SQN, re-authenticating")

		if err := sendAuthRequest(m, ctx, ue, hex.EncodeToString(resp.AUTS), hex.EncodeToString(c.AuthVector.RAND[:])); err != nil {
			logger.From(ctx, logger.MmeLog).Warn("SQN re-synchronisation failed", zap.Error(err))
			rejectAuthentication(m, ctx, ue)
		}

		return nasreply.Handled()
	}

	// The UE attaches with its IMSI, so per TS 24.301 these cases abort with a reject.
	rejectAuthentication(m, ctx, ue)

	return nasreply.Handled()
}

func rejectAuthentication(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	metrics.RegistrationAttempt(metrics.RAT4G, attachTypeName(ue), metrics.ResultReject)

	logger.From(ctx, logger.MmeLog).Info("authentication rejected", zap.String("imsi", ue.IMSI()))
	ue.Conn().SendDownlinkMessage(ctx, &eps.AuthenticationReject{})
	m.ReleaseUEContext(ctx, ue, mme.CauseNASUnspecified)
}
