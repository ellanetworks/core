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

func authenticateOrReject(ctx context.Context, m *mme.MME, ue *mme.UeContext) {
	startAuthentication(ctx, m, ue)
}

func startAuthentication(ctx context.Context, m *mme.MME, ue *mme.UeContext) {
	// resyncTried scopes to one authentication exchange's consecutive synch
	// failures, so a fresh procedure starts with a full budget (TS 24.301 §5.4.2.7).
	ue.Conn().SetResyncTried(false)

	// A new authentication carries an eKSI distinct from the stored one, so the UE keeps
	// its current context usable until the new one is taken into use (TS 24.301 §5.4.2.4).
	ue.SetEksi(mme.NextEksi(ue.Eksi()))

	if err := sendAuthRequest(ctx, m, ue, "", ""); err != nil {
		logger.From(ctx, logger.MmeLog).Info("attach rejected: cannot authenticate subscriber", zap.String("imsi", ue.IMSI()), zap.Error(err))
		rejectAttach(ctx, m, ue, mme.EmmCauseIMSIUnknownInHSS)
	}
}

// sendAuthRequest sends an AUTHENTICATION REQUEST; a set resync pair drives an
// AUTS re-synchronisation.
func sendAuthRequest(ctx context.Context, m *mme.MME, ue *mme.UeContext, resyncAuts, resyncRand string) error {
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
	c.SendGuardedMessage(ctx, "Authentication Request", &eps.AuthenticationRequest{NASKeySetIdentifier: ue.Eksi(), RAND: vec.RAND, AUTN: vec.AUTN[:]})

	return nil
}
