// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"crypto/subtle"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func handleAuthenticationResponse(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	m.StopNASGuard(ue)

	resp, err := eps.ParseAuthenticationResponse(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Authentication Response", zap.Error(err))
		return
	}

	if ue.AuthVector == nil || subtle.ConstantTimeCompare(resp.RES, ue.AuthVector.XRES) != 1 {
		logger.MmeLog.Warn("authentication failed: RES mismatch", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)))
		rejectAuthentication(m, ctx, ue)

		return
	}

	ue.SetKASME(ue.AuthVector.KASME)

	logger.MmeLog.Info("authentication succeeded", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)))
	startSecurityMode(m, ctx, ue)
}
