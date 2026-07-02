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

// handleDetachAccept completes a network-initiated detach: the UE has acknowledged,
// so its context is released and deleted (it is already EMM-DEREGISTERED).
func handleDetachAccept(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	m.StopNASGuard(ue)
	logger.MmeLog.Info("Detach Accept", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)))
	m.ReleaseUEContext(ctx, ue, mme.CauseNASDetach)
}

// handleDetachRequest handles a UE-originating DETACH REQUEST (TS 24.301):
// for a non-switch-off detach it replies with Detach Accept, then releases the
// UE's S1 context.
func handleDetachRequest(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte, integrityVerified bool) {
	// Reject an unprotected DETACH REQUEST while the MME still holds a valid security
	// context: a UE with keys must integrity-protect it, so a forged plain detach
	// cannot deregister an authenticated UE (TS 24.301 §4.4.4.3 defense in depth,
	// mirroring the AMF). A UE that lost its keys can recover via a fresh Attach.
	if !integrityVerified && ue.Secured() {
		logger.MmeLog.Warn("rejecting unauthenticated Detach Request from UE with valid security context",
			zap.String("imsi", ue.IMSI()))

		return
	}

	req, err := eps.ParseDetachRequestUE(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Detach Request", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Detach Request",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
		zap.Bool("switch-off", req.SwitchOff),
		zap.String("imsi", ue.IMSI()),
	)

	ue.SetEMMState(mme.EMMDeregistered)

	if !req.SwitchOff {
		m.SendDownlinkProtected(ctx, ue, &eps.DetachAccept{})
	}

	m.ReleaseUEContext(ctx, ue, mme.CauseNASDetach)
}
