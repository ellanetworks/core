// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/util/timer"
	"go.uber.org/zap"
)

// SendGuardedMessage serializes a NAS message, sends it to the UE, and arms the
// NAS common-procedure guard timer so the message is retransmitted if the UE
// does not respond (TS 24.301: T3450/T3460/T3470).
func (m *MME) SendGuardedMessage(ctx context.Context, ue *UeContext, name string, msg nasMessage) {
	b, err := msg.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	m.SendGuardedDownlink(ctx, ue, name, b)
}

// SendGuardedDownlink sends already-serialized NAS bytes and arms the guard.
func (m *MME) SendGuardedDownlink(ctx context.Context, ue *UeContext, name string, nas []byte) {
	m.ArmNASGuard(ue, name, nas)
	m.SendDownlink(ctx, ue, nas)
}

// ArmNASGuard records the outstanding downlink message and starts its guard
// timer, cancelling any previous one. The retransmitted bytes are kept verbatim
// so the NAS sequence number is preserved across retransmissions (TS 24.301).
// Exhausting the retransmissions releases the UE (the UE stopped answering a
// procedure the network requires).
func (m *MME) ArmNASGuard(ue *UeContext, name string, nas []byte) {
	m.armNASGuardMode(ue, name, nas, nil)
}

// ArmNASGuardAbortOnly arms the guard for a non-critical procedure: exhausting
// the retransmissions runs onAbort and leaves the UE connected rather than
// releasing it (TS 24.301 §6.4.2.5, §6.4.4.5). onAbort finalizes the procedure
// locally (e.g. clearing a pending modification or releasing a single PDN).
func (m *MME) ArmNASGuardAbortOnly(ue *UeContext, name string, nas []byte, onAbort func()) {
	m.armNASGuardMode(ue, name, nas, onAbort)
}

func (m *MME) armNASGuardMode(ue *UeContext, name string, nas []byte, onAbort func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// The NAS guard is connection-scoped; a UE with no S1-connection (ECM-IDLE)
	// has no common procedure to guard.
	if ue.S1 == nil {
		return
	}

	m.stopNASGuardLocked(ue)
	gen := ue.S1.nasGuardGen

	ue.S1.nasGuardName = name
	ue.S1.nasGuardPDU = nas
	ue.S1.nasGuardOnAbort = onAbort

	ue.S1.nasGuardTimer = timer.New(
		m.nasGuardTimeout,
		int32(m.nasGuardMaxRetransmit),
		func(attempt int32) { m.retransmitNASGuard(ue, gen, attempt) },
		func() { m.expireNASGuard(ue, gen) },
	)
}

// StopNASGuard cancels the guard once the UE's response arrives.
func (m *MME) StopNASGuard(ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopNASGuardLocked(ue)
}

// stopNASGuardLocked cancels the guard and invalidates any in-flight callback.
// The caller holds m.mu.
func (m *MME) stopNASGuardLocked(ue *UeContext) {
	if ue.S1 == nil {
		return
	}

	ue.S1.nasGuardGen++

	if ue.S1.nasGuardTimer != nil {
		ue.S1.nasGuardTimer.Stop()
		ue.S1.nasGuardTimer = nil
	}

	ue.S1.nasGuardPDU = nil
	ue.S1.nasGuardOnAbort = nil
}

// retransmitNASGuard resends the outstanding downlink message on each guard
// interval (TS 24.301). gen guards against a timer that fired just before the
// connection was released or re-armed.
func (m *MME) retransmitNASGuard(ue *UeContext, gen uint64, attempt int32) {
	m.mu.Lock()

	if ue.S1 == nil || ue.S1.nasGuardGen != gen {
		m.mu.Unlock()
		return
	}

	pdu := ue.S1.nasGuardPDU
	name := ue.S1.nasGuardName
	mmeUEID := ue.S1.MMEUES1APID

	m.mu.Unlock()

	logger.MmeLog.Info("retransmitting NAS message",
		zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("procedure", name), zap.Int("attempt", int(attempt)))
	// Retransmission is timer-driven, outside the original request; start a fresh root.
	m.SendDownlink(context.Background(), ue, pdu)
}

// expireNASGuard runs once the retransmission limit is exhausted: the UE has
// stopped answering. A critical procedure releases the UE; an abort-only one
// (TS 24.301 §6.4.2.5, §6.4.4.5) runs its finalizer and leaves the UE connected.
func (m *MME) expireNASGuard(ue *UeContext, gen uint64) {
	m.mu.Lock()

	// The connection may have been released (ECM-IDLE) after the timer fired but
	// before it took the lock; the guard goes with it.
	if ue.S1 == nil || ue.S1.nasGuardGen != gen {
		m.mu.Unlock()
		return
	}

	name := ue.S1.nasGuardName
	mmeUEID := ue.S1.MMEUES1APID
	onAbort := ue.S1.nasGuardOnAbort

	ue.S1.nasGuardTimer = nil
	ue.S1.nasGuardPDU = nil
	ue.S1.nasGuardOnAbort = nil
	ue.S1.nasGuardGen++

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
	m.ReleaseUEContext(context.Background(), ue, CauseNASUnspecified)
}
