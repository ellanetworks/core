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
func (c *UeConn) SendGuardedMessage(ctx context.Context, name string, msg nasMessage) {
	if c == nil {
		return
	}

	b, err := msg.MarshalBinary()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal NAS message", zap.Error(err))
		return
	}

	c.SendGuardedDownlink(ctx, name, b)
}

// SendGuardedDownlink sends already-serialized NAS bytes and arms the EMM guard.
func (c *UeConn) SendGuardedDownlink(ctx context.Context, name string, nas []byte) {
	if c == nil {
		return
	}

	c.ArmNASGuard(name, nas)
	c.SendDownlinkNASTransport(ctx, nas)
}

// ArmNASGuard arms the EMM common-procedure guard (T3450/T3460/T3470). EMM
// procedures are mutually exclusive, so the connection has a single EMM guard.
// The retransmitted bytes are kept verbatim so the NAS sequence number is
// preserved (TS 24.301); exhausting the retransmissions releases the UE.
func (c *UeConn) ArmNASGuard(name string, nas []byte) {
	c.armNASGuardMode(name, nas, nil)
}

// ArmNASGuardAbortOnly arms the EMM guard in abort-only mode: exhausting the
// retransmissions runs onAbort, which finalizes the procedure locally and leaves
// the UE connected.
func (c *UeConn) ArmNASGuardAbortOnly(name string, nas []byte, onAbort func()) {
	c.armNASGuardMode(name, nas, onAbort)
}

// ArmT3489 arms the ESM information request guard (TS 24.301 §6.6.1.2.6 a).
func (c *UeConn) ArmT3489(name string, nas []byte, onAbort func()) {
	if c == nil || c.ue == nil {
		return
	}

	m := c.m
	ue := c.ue

	m.mu.Lock()
	defer m.mu.Unlock()

	c.esmInfoGuard.ArmWith(
		c.m.t3489Cfg,
		func(attempt int32) { c.retransmitNASGuard(ue, name, nas, attempt) },
		func() { c.expireNASGuard(ue, name, expiryFinalise, onAbort) },
	)
}

// StopESMInfoGuard cancels the ESM information request guard.
func (c *UeConn) StopESMInfoGuard() {
	if c == nil {
		return
	}

	c.m.mu.Lock()
	defer c.m.mu.Unlock()

	c.esmInfoGuard.Stop()
}

func (c *UeConn) armNASGuardMode(name string, nas []byte, onAbort func()) {
	if c == nil || c.ue == nil {
		return
	}

	m := c.m
	// Capture the UE context at arm time: it stays valid for the callbacks even if the
	// connection is later released (which nils c.ue); the ue.Conn() != c check then
	// invalidates a firing that outlived its connection.
	ue := c.ue

	m.mu.Lock()
	defer m.mu.Unlock()

	c.nasGuardName = name
	c.nasGuard.ArmWith(
		c.m.nasGuardCfg,
		func(attempt int32) { c.retransmitNASGuard(ue, name, nas, attempt) },
		func() { c.expireNASGuard(ue, name, nasGuardExpiry(onAbort), onAbort) },
	)
}

// ArmESMGuard arms p's ESM bearer-procedure guard at its spec interval (T3486
// modify, T3495 deactivate; TS 24.301 §10.2.1). The guard is per-bearer so a UE
// with several PDN connections can have an ESM procedure outstanding on each
// concurrently, and concurrently with an EMM procedure, without cancelling one
// another. (ESM is a 4G MME concept with no 5G AMF analog, so it stays MME-owned.)
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

	conn := ue.Conn()
	if conn == nil {
		return
	}

	p.guard.ArmWith(
		m.esmGuardCfg,
		func(attempt int32) { conn.retransmitNASGuard(ue, name, nas, attempt) },
		func() { conn.expireNASGuard(ue, name, nasGuardExpiry(onAbort), onAbort) },
	)
}

// StopNASGuard cancels the EMM guard.
func (c *UeConn) StopNASGuard() {
	if c == nil {
		return
	}

	c.m.mu.Lock()
	defer c.m.mu.Unlock()

	c.nasGuardName = ""
	c.nasGuard.Stop()
}

// stopNASGuardLocked cancels the EMM guard and invalidates any in-flight callback.
// The caller holds m.mu.
func (m *MME) stopNASGuardLocked(ue *UeContext) {
	conn := ue.Conn()
	if conn == nil {
		return
	}

	conn.nasGuardName = ""
	conn.nasGuard.Stop()
}

// StopESMGuard cancels p's ESM bearer-procedure guard. The guard is
// self-synchronizing, so no registry lock is needed.
func (m *MME) StopESMGuard(p *PdnConnection) {
	p.guard.Stop()
}

// retransmitNASGuard resends the outstanding downlink message on each guard
// interval (TS 24.301). It no-ops if the guarded connection is not the UE's
// current one (released or replaced); the guard already suppresses a firing that
// races a stop or re-arm.
func (c *UeConn) retransmitNASGuard(ue *UeContext, name string, pdu []byte, attempt int32) {
	m := c.m
	m.mu.Lock()

	if ue.Conn() != c {
		m.mu.Unlock()
		return
	}

	mmeUEID := c.MMEUES1APID

	m.mu.Unlock()

	logger.MmeLog.Info("retransmitting NAS message",
		zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("procedure", name), zap.Int("attempt", int(attempt)))
	// Retransmission is timer-driven, outside the original request; start a fresh root.
	c.SendDownlinkNASTransport(context.Background(), pdu)
}

// expireNASGuard runs once the retransmission limit is exhausted. A critical
// procedure releases the UE; an abort-only one (TS 24.301 §6.4.2.5, §6.4.4.5)
// runs its finalizer and leaves the UE connected. It no-ops when the guarded
// connection is not current, so a guard that outlives its connection neither
// releases the UE nor runs its finalizer.
// nasGuardExpiry maps the EMM guard's two arming modes onto the outcome: an
// abort-only arming finalises locally, a plain one releases the UE.
func nasGuardExpiry(onAbort func()) guardExpiry {
	if onAbort != nil {
		return expiryFinalise
	}

	return expiryReleaseUE
}

// guardExpiry says what a guard's exhaustion does: finalise the procedure
// locally, leaving the UE connected, or release the UE.
type guardExpiry uint8

const (
	// expiryReleaseUE ends the UE's connection. It is the EMM common-procedure
	// outcome: the UE never answered a procedure the network requires.
	expiryReleaseUE guardExpiry = iota
	// expiryFinalise runs a per-procedure finaliser and leaves the UE connected.
	expiryFinalise
)

func (c *UeConn) expireNASGuard(ue *UeContext, name string, outcome guardExpiry, onAbort func()) {
	m := c.m
	m.mu.Lock()

	if ue.Conn() != c {
		m.mu.Unlock()
		return
	}

	mmeUEID := c.MMEUES1APID

	m.mu.Unlock()

	if outcome == expiryFinalise {
		logger.MmeLog.Info("NAS procedure timed out, aborting (UE stays connected)",
			zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("procedure", name))

		if onAbort != nil {
			onAbort()
		}

		return
	}

	logger.MmeLog.Info("NAS procedure timed out, releasing UE",
		zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("procedure", name))
	// The guard fires from a timer outside any request; start a fresh root.
	m.ReleaseUEContext(context.Background(), ue, CauseNASUnspecified)
}
