// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

	c.ArmNASGuard(ctx, name, nas, eps.SHTPlain)
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

	c.ArmNASGuard(ctx, name, plain, sht)

	return nil
}

func (c *UeConn) ArmNASGuard(ctx context.Context, name string, plain []byte, sht eps.SecurityHeaderType) {
	c.armNASGuardMode(ctx, name, plain, sht, nil)
}

func (c *UeConn) ArmNASGuardAbortOnly(ctx context.Context, name string, plain []byte, sht eps.SecurityHeaderType, onAbort func(context.Context)) {
	c.armNASGuardMode(ctx, name, plain, sht, onAbort)
}

func (c *UeConn) ArmT3489(ctx context.Context, name string, plain []byte, sht eps.SecurityHeaderType, onAbort func(context.Context)) {
	ue := c.UeContext()
	if ue == nil {
		return
	}

	m := c.m

	m.mu.Lock()
	defer m.mu.Unlock()

	link := trace.SpanContextFromContext(ctx)

	c.esmInfoGuard.ArmWith(
		m.t3489Cfg,
		func(attempt int32) { c.retransmitNASGuard(link, ue, name, plain, sht, attempt) },
		func() { c.expireNASGuard(link, ue, name, onAbort) },
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

func (c *UeConn) armNASGuardMode(ctx context.Context, name string, plain []byte, sht eps.SecurityHeaderType, onAbort func(context.Context)) {
	ue := c.UeContext()
	if ue == nil {
		return
	}

	m := c.m

	m.mu.Lock()
	defer m.mu.Unlock()

	c.nasGuardName = name
	link := trace.SpanContextFromContext(ctx)

	c.nasGuard.ArmWith(
		m.nasGuardCfg,
		func(attempt int32) { c.retransmitNASGuard(link, ue, name, plain, sht, attempt) },
		func() { c.expireNASGuard(link, ue, name, onAbort) },
	)
}

func (m *MME) ArmESMGuard(ctx context.Context, ue *UeContext, p *PdnConnection, name string, plain []byte, sht eps.SecurityHeaderType) {
	m.armESMGuardMode(ctx, ue, p, name, plain, sht, nil)
}

func (m *MME) ArmESMGuardAbortOnly(ctx context.Context, ue *UeContext, p *PdnConnection, name string, plain []byte, sht eps.SecurityHeaderType, onAbort func(context.Context)) {
	m.armESMGuardMode(ctx, ue, p, name, plain, sht, onAbort)
}

func (m *MME) armESMGuardMode(ctx context.Context, ue *UeContext, p *PdnConnection, name string, plain []byte, sht eps.SecurityHeaderType, onAbort func(context.Context)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn := ue.Conn()
	if conn == nil {
		return
	}

	link := trace.SpanContextFromContext(ctx)

	p.guard.ArmWith(
		m.esmGuardCfg,
		func(attempt int32) { conn.retransmitNASGuard(link, ue, name, plain, sht, attempt) },
		func() { conn.expireNASGuard(link, ue, name, onAbort) },
	)
}

// StopNASGuard cancels the EMM guard.
func (c *UeConn) StopNASGuard(ctx context.Context) {
	if c == nil {
		return
	}

	defer c.ResumeDeferredReleaseIfSettled(ctx)

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

func (c *UeConn) retransmitNASGuard(link trace.SpanContext, ue *UeContext, name string, plain []byte, sht eps.SecurityHeaderType, attempt int32) {
	m := c.m
	m.mu.Lock()

	if ue.Conn() != c {
		m.mu.Unlock()
		return
	}

	m.mu.Unlock()

	// Retransmission is timer-driven, outside the original request; start a fresh root.
	ctx, span := guardSpan(link, "mme/nas_guard_retransmit", name, attempt)
	defer span.End()

	c.Log(ctx).Info("retransmitting NAS message",
		zap.String("procedure", name), zap.Int("attempt", int(attempt)))

	if sht == eps.SHTPlain {
		c.SendDownlinkNASTransport(ctx, plain)

		return
	}

	if err := c.SendProtectedNASTransport(ctx, plain, sht); err != nil {
		ReportProtectFailure(ctx, c, name, err)
	}
}

func (c *UeConn) expireNASGuard(link trace.SpanContext, ue *UeContext, name string, onAbort func(context.Context)) {
	m := c.m
	m.mu.Lock()

	if ue.Conn() != c {
		m.mu.Unlock()
		return
	}

	m.mu.Unlock()

	// The guard fires from a timer outside any request; start a fresh root.
	ctx, span := guardSpan(link, "mme/nas_guard_expire", name, 0)
	defer span.End()

	if onAbort != nil {
		c.Log(ctx).Info("NAS procedure timed out, aborting (UE stays connected)",
			zap.String("procedure", name))

		onAbort(ctx)

		return
	}

	c.Log(ctx).Info("NAS procedure timed out, releasing UE", zap.String("procedure", name))
	m.ReleaseUEContext(ctx, ue, CauseNASUnspecified)
}

func guardSpan(link trace.SpanContext, spanName string, timer string, attempt int32) (context.Context, trace.Span) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attribute.String("nas.guard.timer", timer)),
	}

	if attempt > 0 {
		opts = append(opts, trace.WithAttributes(attribute.Int("nas.guard.attempt", int(attempt))))
	}

	if link.IsValid() {
		opts = append(opts, trace.WithLinks(trace.Link{SpanContext: link}))
	}

	return Tracer.Start(context.Background(), spanName, opts...)
}
