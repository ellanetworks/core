// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func authenticateOrReject(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	startAuthentication(m, ctx, ue)
}

func startAuthentication(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	// A fresh authentication procedure gets its own re-synchronisation budget:
	// resyncTried scopes to one exchange's consecutive synch failures (TS 24.301
	// §5.4.2.7).
	ue.Conn().SetResyncTried(false)

	if err := sendAuthRequest(m, ctx, ue, "", ""); err != nil {
		logger.From(ctx, logger.MmeLog).Info("attach rejected: cannot authenticate subscriber", zap.String("imsi", ue.IMSI()), zap.Error(err))
		rejectAttach(m, ctx, ue, mme.EmmCauseIMSIUnknownInHSS)
	}
}

// sendAuthRequest sends an AUTHENTICATION REQUEST; a set resync pair drives an
// AUTS re-synchronisation.
func sendAuthRequest(m *mme.MME, ctx context.Context, ue *mme.UeContext, resyncAuts, resyncRand string) error {
	op, err := m.OperatorPLMN(ctx)
	if err != nil {
		return err
	}

	plmn, err := mme.EncodePLMN(op)
	if err != nil {
		return fmt.Errorf("encode serving PLMN: %w", err)
	}

	vec, err := m.Cred.GenerateEPSVector(ctx, ue.IMSI(), plmn[:], resyncAuts, resyncRand)
	if err != nil {
		return err
	}

	c := ue.Conn()
	c.AuthVector = vec

	logger.From(ctx, logger.MmeLog).Info("Authentication Request")
	c.SendGuardedMessage(ctx, "Authentication Request", &eps.AuthenticationRequest{NASKeySetIdentifier: 0, RAND: vec.RAND, AUTN: vec.AUTN[:]})

	return nil
}
