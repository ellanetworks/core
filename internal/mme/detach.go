// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// DetachUEAfterPathSwitchFailure performs the explicit detach TS 23.401 §5.5.1.1.2
// step 6 requires when no default EPS bearer could be switched during an X2 path
// switch. The UE has already moved to the target cell over the radio and now has no
// usable path there, so no association can carry a NAS DETACH REQUEST: the detach is
// local, and the UE re-attaches (TS 24.301 §5.5.2.3.1).
//
// There is no 5G counterpart: TS 23.502 §4.9.1.2 requires only the PATH SWITCH
// REQUEST FAILURE, leaving the UE's fate to the RAN.
func (m *MME) DetachUEAfterPathSwitchFailure(ctx context.Context, ue *UeContext) {
	if ue == nil {
		return
	}

	logger.From(ctx, logger.MmeLog).Warn("detaching UE: no EPS bearer could be switched during path switch",
		zap.String("imsi", ue.IMSI()))

	ue.TransitionTo(EMMDeregistered)
	m.ReleaseAllSessions(ctx, ue)
	m.RemoveUe(ue)
}

// DetachSubscriber sends a network-initiated DETACH REQUEST (TS 24.301)
// to the attached UE for imsi, if any, when a subscriber is deleted. The
// request is guarded by T3422: if the UE does not reply with Detach Accept it
// is retransmitted, and on exhaustion the UE context is released regardless, so
// a silent UE cannot leak the context.
func (m *MME) DetachSubscriber(ctx context.Context, imsi string) {
	ue, ok := m.LookupUeByIMSI(imsi)
	if !ok {
		return
	}

	// An idle UE (ECM-IDLE) holds no S1 connection to carry the DETACH REQUEST, so
	// release its sessions and context locally. The deleted subscriber fails
	// authentication at its next contact. The connection is snapshotted, not
	// re-read: this runs on the API goroutine, so a concurrent release could nil
	// ue.Conn() between the unlocked calls below.
	ueConn := ue.Conn()
	if ueConn == nil || !m.UeConnected(ue) {
		ue.TransitionTo(EMMDeregistered)
		logger.From(ctx, logger.MmeLog).Info("releasing idle UE on subscriber deletion", zap.String("imsi", imsi))
		m.ReleaseAllSessions(ctx, ue)
		m.RemoveUe(ue)

		return
	}

	// A connected UE with no security context cannot be sent a protected DETACH
	// REQUEST, so perform a local detach. Local detach is spec-recognised
	// (TS 24.301 §5.5.2.3.1): the UE is denied with EMM cause #10 "implicitly
	// detached" at its next contact.
	if !ue.Secured() {
		logger.From(ctx, logger.MmeLog).Info("local detach of connected-but-unsecured UE on subscriber deletion",
			zap.String("imsi", imsi))
		m.ReleaseUEContextLocally(ue, "subscriber deleted")

		return
	}

	// The connected UE is asked to detach; T3422 guards the DETACH REQUEST and the
	// UE stays EMM-DEREGISTERED-INITIATED until it accepts or the guard exhausts.
	ue.TransitionTo(EMMDeregistrationInitiated)

	logger.From(ctx, ueConn.Log).Info("network-initiated detach (subscriber deleted)",
		zap.String("imsi", imsi))

	naspdu, err := ue.ProtectDownlinkMessage(&eps.DetachRequestNetwork{TypeOfDetach: eps.DetachTypeReattachNotRequired})
	if err != nil {
		ReportProtectFailure(ctx, ueConn, "Detach Request", err)
		return
	}

	ueConn.SendDownlinkNASTransport(ctx, naspdu)
	ueConn.ArmNASGuard("Detach Request", naspdu)
}
