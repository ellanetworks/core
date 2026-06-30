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
	// advancing the {NH, NCC} chain. Claim the chain; if a handover holds it, defer
	// the procedure — the UE's attach/TAU retry restarts it once the chain is free.
	if !m.TryClaimKeyChain(ue) {
		logger.MmeLog.Warn("not starting Security Mode Command: a key-changing procedure is in progress (TS 33.401 §7.2.8)",
			zap.String("imsi", ue.IMSI()))

		return
	}

	// The claim is held only while the SECURITY MODE COMMAND is in flight; a failure
	// before it is sent releases it so a later procedure is not blocked. On success
	// it is released by SECURITY MODE COMPLETE or by the connection being freed.
	committed := false

	defer func() {
		if !committed {
			m.ClearKeyChainBusy(ue)
		}
	}()

	// A security policy the MME cannot read must not yield a default (null)
	// context; abort and let the UE retry once the policy is available.
	op, err := m.Bearer.GetOperator(ctx)
	if err != nil {
		logger.MmeLog.Error("failed to resolve operator security policy", zap.Error(err))
		return
	}

	ciphering, err := op.GetCiphering()
	if err != nil {
		logger.MmeLog.Error("failed to read ciphering policy", zap.Error(err))
		return
	}

	integrity, err := op.GetIntegrity()
	if err != nil {
		logger.MmeLog.Error("failed to read integrity policy", zap.Error(err))
		return
	}

	eea, eia, ok := mme.SelectAlgorithms(ue.UeNetCap, ciphering, integrity)
	if !ok {
		logger.MmeLog.Warn("no NAS security algorithm common to UE and operator policy",
			zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
			zap.String("ue-network-capability", fmt.Sprintf("%x", ue.UeNetCap)))
		rejectAttach(m, ctx, ue, mme.EmmCauseUESecCapsMismatch)

		return
	}

	if err := ue.InstallNASSecurityContext(eea, eia, mme.MintAuthProofForSecurityMode()); err != nil {
		logger.MmeLog.Error("failed to install NAS security context", zap.Error(err))
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
		logger.MmeLog.Error("failed to build Security Mode Command", zap.Error(err))
		return
	}

	// Integrity protected with the new EPS security context (TS 24.301).
	wire, err := ue.ProtectDownlink(plain, eps.SHTIntegrityProtectedNewContext)
	if err != nil {
		logger.MmeLog.Error("failed to protect Security Mode Command", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Security Mode Command", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
		zap.Uint8("eea", eea), zap.Uint8("eia", eia),
		zap.String("ue-network-capability", fmt.Sprintf("%x", ue.UeNetCap)),
		zap.String("ms-network-capability", fmt.Sprintf("%x", ue.MsNetCap)),
		zap.String("replayed-ue-security-capability", fmt.Sprintf("%x", replayed)))
	m.SendGuardedDownlink(ctx, ue, "Security Mode Command", wire)

	committed = true
}
