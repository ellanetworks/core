// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func authenticateOrReject(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn) {
	startAuthentication(ctx, m, ue, ueConn)
}

func startAuthentication(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn) {
	// resyncTried scopes to one authentication exchange's consecutive synch
	// failures, so a fresh procedure starts with a full budget (TS 24.301 §5.4.2.7).
	ueConn.SetResyncTried(false)

	// A new authentication carries an eKSI distinct from the stored one, so the UE keeps
	// its current context usable until the new one is taken into use (TS 24.301 §5.4.2.4).
	ue.SetEksi(nas.KeySetIdentifier{Value: mme.NextEksi(ue.Eksi().Value)})

	if err := sendAuthRequest(ctx, m, ue, ueConn, "", ""); err != nil {
		logger.From(ctx, logger.MmeLog).Info("attach rejected: cannot authenticate subscriber", zap.String("imsi", ue.IMSI()), zap.Error(err))
		rejectAttach(ctx, m, ue, ueConn, authRejectCause(err))
	}
}

func authRejectCause(err error) eps.EMMCause {
	if errors.Is(err, db.ErrProposeTimeout) ||
		errors.Is(err, db.ErrOutcomeUnknown) ||
		errors.Is(err, db.ErrMigrationPending) {
		return eps.EMMCauseNetworkFailure
	}

	return eps.EMMCauseIMSIUnknownInHSS
}

// sendAuthRequest sends an AUTHENTICATION REQUEST; a set resync pair drives an
// AUTS re-synchronisation.
func sendAuthRequest(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, resyncAuts, resyncRand string) error {
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

	c := ueConn
	c.AuthVector = vec

	logger.From(ctx, logger.MmeLog).Info("Authentication Request")
	c.SendGuardedMessage(ctx, "Authentication Request", &eps.AuthenticationRequest{NASKeySetIdentifier: ue.Eksi(), RAND: vec.RAND, AUTN: vec.AUTN})

	return nil
}
