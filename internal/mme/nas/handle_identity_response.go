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
	m.StopNASGuard(ue)

	resp, err := eps.ParseIdentityResponse(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Identity Response", zap.Error(err))
		return
	}

	m.SetIMSI(ue, mobileIdentityDigits(resp.MobileIdentity))
	authenticateOrReject(m, ctx, ue)
}
