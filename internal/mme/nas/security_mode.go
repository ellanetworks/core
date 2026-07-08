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

func startSecurityMode(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	// TS 33.501 §6.9.5.1 / TS 33.401 §7.2.8: the security mode procedure re-keys the
	// AS context, so it must not run concurrently with an S1 handover or Path Switch
	// advancing the {NH, NCC} chain. Claim the chain; if a handover holds it, defer.
	if !m.TryClaimKeyChain(ue) {
		logger.From(ctx, logger.MmeLog).Warn("not starting Security Mode Command: a key-changing procedure is in progress (TS 33.401 §7.2.8)",
			zap.String("imsi", ue.IMSI()))

		return
	}

	// Release the claim if the SECURITY MODE COMMAND is not sent, so a failure before
	// send does not block a later key-changing procedure. Once sent, the claim is
	// released by SECURITY MODE COMPLETE or by the connection being freed.
	committed := false

	defer func() {
		if !committed {
			m.ClearKeyChainBusy(ue)
		}
	}()

	// A security policy the MME cannot read must not yield a default (null) context.
	intOrder, encOrder, err := m.SecurityAlgorithms(ctx)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to resolve operator security policy", zap.Error(err))
		return
	}

	eea, eia, ok := mme.SelectAlgorithms(ue.UeNetCap, intOrder, encOrder)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("no NAS security algorithm common to UE and operator policy",
			zap.String("ue-network-capability", fmt.Sprintf("%x", ue.UeNetCap)))
		rejectAttach(m, ctx, ue, mme.EmmCauseUESecCapsMismatch)

		return
	}

	if err := ue.InstallNASSecurityContext(eea, eia, mme.MintAuthProofForSecurityMode()); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to install NAS security context", zap.Error(err))
		return
	}

	replayed := mme.ReplayedUESecCap(ue.UeNetCap, ue.MsNetCap)

	smc := &eps.SecurityModeCommand{
		CipheringAlgorithm:             eea,
		IntegrityAlgorithm:             eia,
		NASKeySetIdentifier:            0,
		ReplayedUESecurityCapabilities: replayed,
		IMEISVRequested:                true,
		HASHMME:                        mme.HashMME(ue.HashmmeInput),
	}

	plain, err := smc.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Security Mode Command", zap.Error(err))
		return
	}

	wire, err := ue.ProtectDownlink(plain, eps.SHTIntegrityProtectedNewContext)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to protect Security Mode Command", zap.Error(err))
		return
	}

	logger.From(ctx, logger.MmeLog).Info("Security Mode Command",
		zap.Uint8("eea", eea), zap.Uint8("eia", eia),
		zap.String("ue-network-capability", fmt.Sprintf("%x", ue.UeNetCap)),
		zap.String("ms-network-capability", fmt.Sprintf("%x", ue.MsNetCap)),
		zap.String("replayed-ue-security-capability", fmt.Sprintf("%x", replayed)))
	ue.AdvanceRegStep(mme.RegStepSecurityMode)
	ue.Conn().SendGuardedDownlink(ctx, "Security Mode Command", wire)

	committed = true
}
