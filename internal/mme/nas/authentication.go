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

// startAuthentication challenges the UE with an EPS-AKA vector. A subscriber the
// credential authority cannot serve (e.g. an unknown IMSI) is rejected with
// ATTACH REJECT #2.
func startAuthentication(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	if err := sendAuthRequest(m, ctx, ue, "", ""); err != nil {
		logger.MmeLog.Info("attach rejected: cannot authenticate subscriber",
			zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)), zap.String("imsi", ue.IMSI()), zap.Error(err))
		rejectAttach(m, ctx, ue, mme.EmmCauseIMSIUnknownInHSS)
	}
}

// sendAuthRequest obtains an EPS-AKA vector from the credential authority and
// sends an AUTHENTICATION REQUEST. A set resync pair drives an AUTS
// re-synchronisation.
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

	ue.AuthVector = vec

	logger.MmeLog.Info("Authentication Request", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)))
	m.SendGuardedMessage(ctx, ue, "Authentication Request", &eps.AuthenticationRequest{NASKeySetIdentifier: 0, RAND: vec.RAND, AUTN: vec.AUTN[:]})

	return nil
}
