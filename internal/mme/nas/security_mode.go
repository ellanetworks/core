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

type keyInstall uint8

const (
	freshKeys keyInstall = iota
	rekeyedKeys
)

func startSecurityMode(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, install keyInstall) {
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

	eea, eia, ok := eps.SelectNASAlgorithms(ue.UeNetCap(), intOrder, encOrder)
	if !ok {
		logger.From(ctx, logger.MmeLog).Warn("no NAS security algorithm common to UE and operator policy",
			zap.Stringer("ue-network-capability", ue.UeNetCap()))
		rejectAttach(ctx, m, ue, ueConn, eps.EMMCauseUESecurityCapabilitiesMismatch)

		return
	}

	installKeys := ue.InstallNASSecurityContext
	if install == rekeyedKeys {
		installKeys = ue.RekeyNASSecurityContext
	}

	if err := installKeys(eea, eia, mme.MintAuthProofForSecurityMode()); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to install NAS security context", zap.Error(err))
		return
	}

	imeisvRequested := eps.IMEISVRequested

	smc := &eps.SecurityModeCommand{
		CipheringAlgorithm:           eea,
		IntegrityAlgorithm:           eia,
		NASKeySetIdentifier:          ue.Eksi(),
		ReplayedUESecurityCapability: eps.ReplayedUESecurityCapability(ue.UeNetCap(), ue.MsNetCap()),
		IMEISVRequested:              &imeisvRequested,
		HASHMME:                      mme.HashMME(ue.HashmmeInput),
	}

	plain, err := smc.MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to build Security Mode Command", zap.Error(err))
		return
	}

	logger.From(ctx, logger.MmeLog).Info("Security Mode Command",
		zap.Stringer("eea", eea), zap.Stringer("eia", eia),
		zap.Stringer("ue-network-capability", ue.UeNetCap()),
		zap.Any("ms-network-capability", ue.MsNetCap()),
		zap.Stringer("replayed-ue-security-capability", smc.ReplayedUESecurityCapability))

	if err := ueConn.SendGuardedProtected(ctx, "Security Mode Command", plain, eps.SHTIntegrityProtectedNewContext); err != nil {
		return
	}

	ue.AdvanceRegStep(mme.RegStepSecurityMode)

	committed = true
}
