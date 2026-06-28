// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// detachTypeReattachNotRequired is the network-originating detach type meaning
// the UE shall not re-attach (TS 24.301) — used when a subscriber is
// removed.
const detachTypeReattachNotRequired uint8 = 2

// DetachSubscriber sends a network-initiated DETACH REQUEST (TS 24.301)
// to the attached UE for imsi, if any, when a subscriber is deleted. The
// request is guarded by T3422: if the UE does not reply with Detach Accept it
// is retransmitted, and on exhaustion the UE context is released regardless, so
// a silent UE cannot leak the context.
func (m *MME) DetachSubscriber(ctx context.Context, imsi string) {
	ue, ok := m.lookupUeByIMSI(imsi)
	if !ok {
		return
	}

	ue.emmState.store(EMMDeregistered)

	// An idle UE (ECM-IDLE) holds no S1 connection to carry the DETACH REQUEST, so
	// release its sessions and context locally. The deleted subscriber is denied at
	// its next contact, as it can no longer authenticate.
	m.mu.RLock()

	connected := ue.s1 != nil

	m.mu.RUnlock()

	if !connected {
		logger.MmeLog.Info("releasing idle UE on subscriber deletion", zap.String("imsi", imsi))
		m.releaseAllSessions(ue)
		m.removeUe(ue)

		return
	}

	logger.MmeLog.Info("network-initiated detach (subscriber deleted)",
		zap.Uint32("mme-ue-id", uint32(ue.s1.MMEUES1APID)), zap.String("imsi", imsi))

	naspdu, err := m.protectDownlink(ue, &eps.DetachRequestNetwork{TypeOfDetach: detachTypeReattachNotRequired})
	if err != nil {
		logger.MmeLog.Error("failed to protect Detach Request", zap.Error(err))
		return
	}

	m.sendDownlink(ctx, ue, naspdu)
	m.armNASGuard(ue, "Detach Request", naspdu)
}

// handleDetachAccept completes a network-initiated detach: the UE has acknowledged,
// so stop the guard and release and delete its context (the UE is already
// EMM-DEREGISTERED).
func (m *MME) handleDetachAccept(ctx context.Context, ue *UeContext) {
	m.stopNASGuard(ue)
	logger.MmeLog.Info("Detach Accept", zap.Uint32("mme-ue-id", uint32(ue.s1.MMEUES1APID)))
	m.releaseUEContext(ctx, ue, causeNASDetach)
}

// isSwitchOffDetach reports whether body is a plain UE-originating DETACH
// REQUEST with the switch-off flag set — the one NAS message the MME accepts
// without integrity protection (TS 24.301).
func isSwitchOffDetach(body []byte) bool {
	if mt, err := eps.PeekMessageType(body); err != nil || mt != eps.MsgDetachRequest {
		return false
	}

	req, err := eps.ParseDetachRequestUE(body)

	return err == nil && req.SwitchOff
}

// handleDetachRequest handles a UE-originating DETACH REQUEST (TS 24.301):
// for a non-switch-off detach it replies with Detach Accept, then releases the
// UE's S1 context.
func (m *MME) handleDetachRequest(ctx context.Context, ue *UeContext, plain []byte) {
	req, err := eps.ParseDetachRequestUE(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Detach Request", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Detach Request",
		zap.Uint32("mme-ue-id", uint32(ue.s1.MMEUES1APID)),
		zap.Bool("switch-off", req.SwitchOff),
		zap.String("imsi", ue.imsi),
	)

	ue.emmState.store(EMMDeregistered)

	if !req.SwitchOff {
		m.sendDownlinkProtected(ctx, ue, &eps.DetachAccept{})
	}

	m.releaseUEContext(ctx, ue, causeNASDetach)
}
