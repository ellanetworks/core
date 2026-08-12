// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

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

func (c *UeConn) SendGuardedDownlink(ctx context.Context, name string, nas []byte) {
	if c == nil {
		return
	}

	c.ArmNASGuard(name, nas, eps.SHTPlain)
	c.SendDownlinkNASTransport(ctx, nas)
}

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

func (c *UeConn) ArmNASGuard(name string, plain []byte, sht eps.SecurityHeaderType) {
	c.armNASGuardMode(name, plain, sht, nil)
}

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

func (m *MME) ArmESMGuard(ue *UeContext, p *PdnConnection, name string, plain []byte, sht eps.SecurityHeaderType) {
	m.armESMGuardMode(ue, p, name, plain, sht, nil)
}

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

func (m *MME) StopESMGuard(p *PdnConnection) {
	p.guard.Stop()
}

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
