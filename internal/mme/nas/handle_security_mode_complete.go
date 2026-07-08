// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func handleSecurityModeComplete(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	// A SECURITY MODE COMPLETE is valid only during the security mode sub-phase;
	// out of order, ignore it. A genuine one is integrity-protected against the
	// context installed at command send, so this is defence in depth.
	if ue.RegStep() != mme.RegStepSecurityMode {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Security Mode Complete outside the security mode sub-phase")

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ue.Conn().StopNASGuard()
	// Release the key-chain claim so a subsequent handover or Path Switch can
	// proceed (TS 33.401 §7.2.8).
	m.ClearKeyChainBusy(ue)

	smc, err := eps.ParseSecurityModeComplete(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Security Mode Complete", zap.Error(err))
		return nasreply.Handled()
	}

	// The Security Mode Command requested the IMEISV (TS 24.301); parse it into the
	// shared equipment-identity type for the status API.
	var pei etsi.IMEI

	if len(smc.IMEISV) > 0 {
		if parsed, err := etsi.NewIMEIFromPEI("imeisv-" + mobileIdentityDigits(smc.IMEISV)); err == nil {
			pei = parsed
		} else {
			logger.From(ctx, logger.MmeLog).Warn("failed to parse IMEISV", zap.String("imsi", ue.IMSI()), zap.Error(err))
		}
	}

	ue.MarkSecured(pei)

	// Anti-tamper recovery: on a HASHMME mismatch the UE returns the complete plain
	// ATTACH REQUEST in the Replayed NAS message container. Re-ingest it so a tampered
	// initial Attach cannot alter the completed attach (TS 24.301 §5.4.3.4).
	if len(smc.ReplayedNASMessage) > 0 {
		req, err := eps.ParseAttachRequest(smc.ReplayedNASMessage)
		if err != nil {
			logger.From(ctx, logger.MmeLog).Warn("failed to decode replayed NAS message container in Security Mode Complete", zap.Error(err))
			return nasreply.Handled()
		}

		logger.From(ctx, logger.MmeLog).Info("recovered genuine Attach Request from replayed NAS message container", zap.String("imsi", ue.IMSI()))

		ingestAttachRequest(ue, req)
	}

	logger.From(ctx, logger.MmeLog).Info("NAS security context established",
		zap.String("imsi", ue.IMSI()),
	)

	activateDefaultBearer(m, ctx, ue)

	return nasreply.Handled()
}
