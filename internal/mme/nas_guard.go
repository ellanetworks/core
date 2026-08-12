// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
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

// SendGuardedDownlink sends already-serialized plain NAS bytes and arms the EMM
// guard. For the messages that precede security activation; a protected message
// goes through SendGuardedProtected.
func (c *UeConn) SendGuardedDownlink(ctx context.Context, name string, nas []byte) {
	if c == nil {
		return
	}

	c.ArmNASGuard(name, nas, eps.SHTPlain)
	c.SendDownlinkNASTransport(ctx, nas)
}

// SendGuardedProtected protects plain, sends it in a Downlink NAS Transport, and
// arms the EMM guard on the plain message, so each retransmission is protected
// afresh under the next downlink NAS COUNT (TS 24.301 §4.4.3.1).
func (c *UeConn) SendGuardedProtected(ctx context.Context, name string, plain []byte, sht eps.SecurityHeaderType) error {
	if c == nil {
		return nil
	}

	if err := c.SendProtectedNASTransport(ctx, plain, sht); err != nil {
		ReportProtectFailure(ctx, c, name, err)

		return err
	}

	c.ArmNASGuard(name, plain, sht)

	return nil
}

// ArmNASGuard arms the EMM common-procedure guard (T3450/T3460/T3470). EMM
// procedures are mutually exclusive, so the connection has a single EMM guard.
// The plain message is kept, and each retransmission of a protected one takes the
// next NAS COUNT (TS 24.301 §4.4.3.1); a security header type of eps.SHTPlain
// marks a message sent unprotected. Exhausting the retransmissions releases the UE.
func (c *UeConn) ArmNASGuard(name string, plain []byte, sht eps.SecurityHeaderType) {
	c.armNASGuardMode(name, plain, sht, nil)
}

// ArmNASGuardAbortOnly arms the EMM guard in abort-only mode: exhausting the
// retransmissions runs onAbort, which finalizes the procedure locally and leaves
// the UE connected.
func (c *UeConn) ArmNASGuardAbortOnly(name string, plain []byte, sht eps.SecurityHeaderType, onAbort func()) {
	c.armNASGuardMode(name, plain, sht, onAbort)
}

func (c *UeConn) ArmT3489(name string, plain []byte, sht eps.SecurityHeaderType, onAbort func()) {
	if c == nil || c.ue == nil {
		return
	}

	m := c.m
	ue := c.ue

	m.mu.Lock()
	defer m.mu.Unlock()

	c.esmInfoGuard.ArmWith(
		m.t3489Cfg,
		func(attempt int32) { c.retransmitNASGuard(ue, name, plain, sht, attempt) },
		func() { c.expireNASGuard(ue, name, onAbort) },
	)
}

func (c *UeConn) StopESMInfoGuard() {
	if c == nil {
		return
	}

	c.m.mu.Lock()
	defer c.m.mu.Unlock()

	c.esmInfoGuard.Stop()
}

func (c *UeConn) armNASGuardMode(name string, plain []byte, sht eps.SecurityHeaderType, onAbort func()) {
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
		m.nasGuardCfg,
		func(attempt int32) { c.retransmitNASGuard(ue, name, plain, sht, attempt) },
		func() { c.expireNASGuard(ue, name, onAbort) },
	)
}

// ArmESMGuard arms p's ESM bearer-procedure guard at its spec interval (T3486
// modify, T3495 deactivate; TS 24.301 §10.2.1). The guard is per-bearer so a UE
// with several PDN connections can have an ESM procedure outstanding on each
// concurrently, and concurrently with an EMM procedure, without cancelling one
// another. (ESM is a 4G MME concept with no 5G AMF analog, so it stays MME-owned.)
func (m *MME) ArmESMGuard(ue *UeContext, p *PdnConnection, name string, plain []byte, sht eps.SecurityHeaderType) {
	m.armESMGuardMode(ue, p, name, plain, sht, nil)
}

// ArmESMGuardAbortOnly arms p's ESM bearer-procedure guard in abort-only mode: on
// exhaustion onAbort finalizes the procedure locally and the UE stays connected
// (TS 24.301 §6.4.2.5 modify, §6.4.4.5 deactivate).
func (m *MME) ArmESMGuardAbortOnly(ue *UeContext, p *PdnConnection, name string, plain []byte, sht eps.SecurityHeaderType, onAbort func()) {
	m.armESMGuardMode(ue, p, name, plain, sht, onAbort)
}

func (m *MME) armESMGuardMode(ue *UeContext, p *PdnConnection, name string, plain []byte, sht eps.SecurityHeaderType, onAbort func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn := ue.Conn()
	if conn == nil {
		return
	}

	p.guard.ArmWith(
		m.esmGuardCfg,
		func(attempt int32) { conn.retransmitNASGuard(ue, name, plain, sht, attempt) },
		func() { conn.expireNASGuard(ue, name, onAbort) },
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
// interval (TS 24.301). A protected message is protected again under the next
// downlink NAS COUNT, which the sender increases after each new or retransmitted
// outbound protected message (TS 24.301 §4.4.3.1). It no-ops if the guarded
// connection is not the UE's current one (released or replaced); the guard already
// suppresses a firing that races a stop or re-arm.
func (c *UeConn) retransmitNASGuard(ue *UeContext, name string, plain []byte, sht eps.SecurityHeaderType, attempt int32) {
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
	ctx := context.Background()

	if sht == eps.SHTPlain {
		c.SendDownlinkNASTransport(ctx, plain)

		return
	}

	if err := c.SendProtectedNASTransport(ctx, plain, sht); err != nil {
		ReportProtectFailure(ctx, c, name, err)
	}
}

// expireNASGuard runs once the retransmission limit is exhausted. A critical
// procedure releases the UE; an abort-only one (TS 24.301 §6.4.2.5, §6.4.4.5)
// runs its finalizer and leaves the UE connected. It no-ops when the guarded
// connection is not current, so a guard that outlives its connection neither
// releases the UE nor runs its finalizer.
func (c *UeConn) expireNASGuard(ue *UeContext, name string, onAbort func()) {
	m := c.m
	m.mu.Lock()

	if ue.Conn() != c {
		m.mu.Unlock()
		return
	}

	mmeUEID := c.MMEUES1APID

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
