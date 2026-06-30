// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// SendGuardedMessage serializes a NAS message, sends it to the UE, and arms the
// EMM common-procedure guard so the message is retransmitted if the UE does not
// respond (TS 24.301: T3450/T3460/T3470).
func (m *MME) SendGuardedMessage(ctx context.Context, ue *UeContext, name string, msg nasMessage) {
	b, err := msg.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	m.SendGuardedDownlink(ctx, ue, name, b)
}

// SendGuardedDownlink sends already-serialized NAS bytes and arms the EMM guard.
func (m *MME) SendGuardedDownlink(ctx context.Context, ue *UeContext, name string, nas []byte) {
	m.ArmNASGuard(ue, name, nas)
	m.SendDownlink(ctx, ue, nas)
}

// ArmNASGuard arms the EMM common-procedure guard (T3450/T3460/T3470). EMM
// procedures are mutually exclusive, so the connection has a single EMM guard.
// The retransmitted bytes are kept verbatim so the NAS sequence number is
// preserved (TS 24.301); exhausting the retransmissions releases the UE.
func (m *MME) ArmNASGuard(ue *UeContext, name string, nas []byte) {
	m.armNASGuardMode(ue, name, nas, nil)
}

// ArmNASGuardAbortOnly arms the EMM guard in abort-only mode: exhausting the
// retransmissions runs onAbort and leaves the UE connected rather than releasing
// it. onAbort finalizes the procedure locally.
func (m *MME) ArmNASGuardAbortOnly(ue *UeContext, name string, nas []byte, onAbort func()) {
	m.armNASGuardMode(ue, name, nas, onAbort)
}

func (m *MME) armNASGuardMode(ue *UeContext, name string, nas []byte, onAbort func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// The EMM guard is connection-scoped; a UE with no S1-connection (ECM-IDLE)
	// has no procedure to guard.
	conn := ue.S1
	if conn == nil {
		return
	}

	conn.nasGuard.Arm(
		m.nasGuardTimeout,
		int32(m.nasGuardMaxRetransmit),
		func(attempt int32) { m.retransmitNASGuard(ue, conn, name, nas, attempt) },
		func() { m.expireNASGuard(ue, conn, name, onAbort) },
	)
}

// ArmESMGuard arms p's ESM bearer-procedure guard at its spec interval (T3486
// modify, T3495 deactivate; TS 24.301 §10.2.1). The guard is per-bearer so a UE
// with several PDN connections can have an ESM procedure outstanding on each
// concurrently, and concurrently with an EMM procedure, without cancelling one
// another.
func (m *MME) ArmESMGuard(ue *UeContext, p *PdnConnection, name string, nas []byte) {
	m.armESMGuardMode(ue, p, name, nas, nil)
}

// ArmESMGuardAbortOnly arms p's ESM bearer-procedure guard in abort-only mode: on
// exhaustion onAbort finalizes the procedure locally and the UE stays connected
// (TS 24.301 §6.4.2.5 modify, §6.4.4.5 deactivate).
func (m *MME) ArmESMGuardAbortOnly(ue *UeContext, p *PdnConnection, name string, nas []byte, onAbort func()) {
	m.armESMGuardMode(ue, p, name, nas, onAbort)
}

func (m *MME) armESMGuardMode(ue *UeContext, p *PdnConnection, name string, nas []byte, onAbort func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn := ue.S1
	if conn == nil {
		return
	}

	p.guard.Arm(
		m.esmGuardTimeout,
		int32(m.nasGuardMaxRetransmit),
		func(attempt int32) { m.retransmitNASGuard(ue, conn, name, nas, attempt) },
		func() { m.expireNASGuard(ue, conn, name, onAbort) },
	)
}

// StopNASGuard cancels the EMM guard once the UE's response arrives.
func (m *MME) StopNASGuard(ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopNASGuardLocked(ue)
}

// stopNASGuardLocked cancels the EMM guard and invalidates any in-flight callback.
// The caller holds m.mu.
func (m *MME) stopNASGuardLocked(ue *UeContext) {
	if ue.S1 == nil {
		return
	}

	ue.S1.nasGuard.Stop()
}

// StopESMGuard cancels p's ESM bearer-procedure guard once the UE answers. The
// guard is self-synchronizing, so no registry lock is needed.
func (m *MME) StopESMGuard(p *PdnConnection) {
	p.guard.Stop()
}

// retransmitNASGuard resends the outstanding downlink message on each guard
// interval (TS 24.301). It no-ops if the connection it guarded is no longer the
// UE's current one (released or replaced); the guard already suppresses a firing
// that races a stop or re-arm.
func (m *MME) retransmitNASGuard(ue *UeContext, conn *S1Conn, name string, pdu []byte, attempt int32) {
	m.mu.Lock()

	if ue.S1 != conn {
		m.mu.Unlock()
		return
	}

	mmeUEID := conn.MMEUES1APID

	m.mu.Unlock()

	logger.MmeLog.Info("retransmitting NAS message",
		zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("procedure", name), zap.Int("attempt", int(attempt)))
	// Retransmission is timer-driven, outside the original request; start a fresh root.
	m.SendDownlink(context.Background(), ue, pdu)
}

// expireNASGuard runs once the retransmission limit is exhausted: the UE has
// stopped answering. A critical procedure releases the UE; an abort-only one
// (TS 24.301 §6.4.2.5, §6.4.4.5) runs its finalizer and leaves the UE connected.
// It no-ops if the connection it guarded is no longer current, so a guard that
// outlives its connection neither releases the UE nor runs its finalizer.
func (m *MME) expireNASGuard(ue *UeContext, conn *S1Conn, name string, onAbort func()) {
	m.mu.Lock()

	if ue.S1 != conn {
		m.mu.Unlock()
		return
	}

	mmeUEID := conn.MMEUES1APID

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
