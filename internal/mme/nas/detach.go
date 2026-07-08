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

// releaseDetachSessions releases the UE's EPS sessions up front on detach so the
// UPF drops any remaining packets and frees the tunnel immediately (TS 23.401
// §5.3.8.2.1), rather than buffering them for paging as the shared S1 release path
// does for an idle transition. ReleasePDN removes the PDN, so the DeactivateAllSessions
// run on UE Context Release Complete then finds nothing to buffer.
func releaseDetachSessions(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	for _, p := range m.SnapshotPDNs(ue) {
		m.ReleasePDN(ctx, ue, p)
	}
}

func handleDetachAccept(m *mme.MME, ctx context.Context, ue *mme.UeContext) {
	ue.Conn().StopNASGuard()
	logger.From(ctx, logger.MmeLog).Info("Detach Accept")
	releaseDetachSessions(m, ctx, ue)
	m.ReleaseUEContext(ctx, ue, mme.CauseNASDetach)
}

func handleDetachRequest(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte, integrityVerified bool) {
	// A UE holding keys must integrity-protect its DETACH REQUEST, so a forged plain
	// detach cannot deregister an authenticated UE (TS 24.301 §4.4.4.3 defence in
	// depth). A UE that lost its keys can recover via a fresh Attach.
	if !integrityVerified && ue.Secured() {
		logger.From(ctx, logger.MmeLog).Warn("rejecting unauthenticated Detach Request from UE with valid security context",
			zap.String("imsi", ue.IMSI()))

		return
	}

	req, err := eps.ParseDetachRequestUE(plain)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Detach Request", zap.Error(err))
		return
	}

	logger.From(ctx, logger.MmeLog).Info("Detach Request",
		zap.Bool("switch-off", req.SwitchOff),
		zap.String("imsi", ue.IMSI()),
	)

	ue.TransitionTo(mme.EMMDeregistered)

	// Release the user plane before acknowledging the detach, so the UPF has stopped
	// forwarding by the time the UE acts on the DETACH ACCEPT (TS 23.401 §5.3.8.2.1);
	// acknowledging first would leave a window where the released UE can still pass
	// traffic.
	releaseDetachSessions(m, ctx, ue)

	if !req.SwitchOff {
		ue.Conn().SendDownlinkProtected(ctx, &eps.DetachAccept{})
	}

	m.ReleaseUEContext(ctx, ue, mme.CauseNASDetach)
}
