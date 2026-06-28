// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/util/timer"
	"go.uber.org/zap"
)

// sendGuardedMessage serializes a NAS message, sends it to the UE, and arms the
// NAS common-procedure guard timer so the message is retransmitted if the UE
// does not respond (TS 24.301: T3450/T3460/T3470).
func (m *MME) sendGuardedMessage(ctx context.Context, ue *UeContext, name string, msg nasMessage) {
	b, err := msg.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	m.sendGuardedDownlink(ctx, ue, name, b)
}

// sendGuardedDownlink sends already-serialized NAS bytes and arms the guard.
func (m *MME) sendGuardedDownlink(ctx context.Context, ue *UeContext, name string, nas []byte) {
	m.armNASGuard(ue, name, nas)
	m.sendDownlink(ctx, ue, nas)
}

// armNASGuard records the outstanding downlink message and starts its guard
// timer, cancelling any previous one. The retransmitted bytes are kept verbatim
// so the NAS sequence number is preserved across retransmissions (TS 24.301).
// Exhausting the retransmissions releases the UE (the UE stopped answering a
// procedure the network requires).
func (m *MME) armNASGuard(ue *UeContext, name string, nas []byte) {
	m.armNASGuardMode(ue, name, nas, nil)
}

// armNASGuardAbortOnly arms the guard for a non-critical procedure: exhausting
// the retransmissions runs onAbort and leaves the UE connected rather than
// releasing it (TS 24.301 §6.4.2.5, §6.4.4.5). onAbort finalizes the procedure
// locally (e.g. clearing a pending modification or releasing a single PDN).
func (m *MME) armNASGuardAbortOnly(ue *UeContext, name string, nas []byte, onAbort func()) {
	m.armNASGuardMode(ue, name, nas, onAbort)
}

func (m *MME) armNASGuardMode(ue *UeContext, name string, nas []byte, onAbort func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// The NAS guard is connection-scoped; a UE with no S1-connection (ECM-IDLE)
	// has no common procedure to guard.
	if ue.s1 == nil {
		return
	}

	m.stopNASGuardLocked(ue)
	gen := ue.s1.nasGuardGen

	ue.s1.nasGuardName = name
	ue.s1.nasGuardPDU = nas
	ue.s1.nasGuardOnAbort = onAbort

	ue.s1.nasGuardTimer = timer.New(
		m.nasGuardTimeout,
		int32(m.nasGuardMaxRetransmit),
		func(attempt int32) { m.retransmitNASGuard(ue, gen, attempt) },
		func() { m.expireNASGuard(ue, gen) },
	)
}

// stopNASGuard cancels the guard once the UE's response arrives.
func (m *MME) stopNASGuard(ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopNASGuardLocked(ue)
}

// stopNASGuardLocked cancels the guard and invalidates any in-flight callback.
// The caller holds m.mu.
func (m *MME) stopNASGuardLocked(ue *UeContext) {
	if ue.s1 == nil {
		return
	}

	ue.s1.nasGuardGen++

	if ue.s1.nasGuardTimer != nil {
		ue.s1.nasGuardTimer.Stop()
		ue.s1.nasGuardTimer = nil
	}

	ue.s1.nasGuardPDU = nil
	ue.s1.nasGuardOnAbort = nil
}

// retransmitNASGuard resends the outstanding downlink message on each guard
// interval (TS 24.301). gen guards against a timer that fired just before the
// connection was released or re-armed.
func (m *MME) retransmitNASGuard(ue *UeContext, gen uint64, attempt int32) {
	m.mu.Lock()

	if ue.s1 == nil || ue.s1.nasGuardGen != gen {
		m.mu.Unlock()
		return
	}

	pdu := ue.s1.nasGuardPDU
	name := ue.s1.nasGuardName
	mmeUEID := ue.s1.MMEUES1APID

	m.mu.Unlock()

	logger.MmeLog.Info("retransmitting NAS message",
		zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("procedure", name), zap.Int("attempt", int(attempt)))
	// Retransmission is timer-driven, outside the original request; start a fresh root.
	m.sendDownlink(context.Background(), ue, pdu)
}

// expireNASGuard runs once the retransmission limit is exhausted: the UE has
// stopped answering. A critical procedure releases the UE; an abort-only one
// (TS 24.301 §6.4.2.5, §6.4.4.5) runs its finalizer and leaves the UE connected.
func (m *MME) expireNASGuard(ue *UeContext, gen uint64) {
	m.mu.Lock()

	// The connection may have been released (ECM-IDLE) after the timer fired but
	// before it took the lock; the guard goes with it.
	if ue.s1 == nil || ue.s1.nasGuardGen != gen {
		m.mu.Unlock()
		return
	}

	name := ue.s1.nasGuardName
	mmeUEID := ue.s1.MMEUES1APID
	onAbort := ue.s1.nasGuardOnAbort

	ue.s1.nasGuardTimer = nil
	ue.s1.nasGuardPDU = nil
	ue.s1.nasGuardOnAbort = nil
	ue.s1.nasGuardGen++

	m.mu.Unlock()

	if onAbort != nil {
		logger.MmeLog.Info("NAS procedure timed out, aborting (UE stays connected)",
			zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("procedure", name))

		onAbort()

		return
	}

	logger.MmeLog.Info("NAS procedure timed out, releasing UE",
		zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("procedure", name))
	// The guard fires from a timer outside any request; start a fresh root.
	m.releaseUEContext(context.Background(), ue, causeNASUnspecified)
}
