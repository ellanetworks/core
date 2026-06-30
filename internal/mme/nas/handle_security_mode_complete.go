// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func handleSecurityModeComplete(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	m.StopNASGuard(ue)
	// The security mode procedure is complete; release the key-chain claim so a
	// subsequent handover or Path Switch can proceed (TS 33.401 §7.2.8).
	m.ClearKeyChainBusy(ue)

	smc, err := eps.ParseSecurityModeComplete(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Security Mode Complete", zap.Error(err))
		return
	}

	// The UE returns its IMEISV when requested in the Security Mode Command
	// (TS 24.301). Convert it to a 15-digit IMEI for the status API.
	var imei string

	if len(smc.IMEISV) > 0 {
		if derived, err := etsi.IMEIFromPEI("imeisv-" + mobileIdentityDigits(smc.IMEISV)); err == nil {
			imei = derived
		} else {
			logger.MmeLog.Warn("failed to derive IMEI from IMEISV", zap.String("imsi", ue.IMSI()), zap.Error(err))
		}
	}

	ue.MarkSecured(imei)

	logger.MmeLog.Info("NAS security context established",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
		zap.String("imsi", ue.IMSI()),
	)

	activateDefaultBearer(m, ctx, ue)
}
