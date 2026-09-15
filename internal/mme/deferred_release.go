// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/s1ap"
	"go.opentelemetry.io/otel/trace"
)

const deferredReleaseTimeout = 30 * time.Second

func (c *UeConn) MTSignallingPending() bool {
	if c == nil {
		return false
	}

	if c.UeContext().MTDeliveryInProgress() {
		return true
	}

	if c.ICS() == ICSPending {
		return true
	}

	return c.nasGuard.Active() || c.esmInfoGuard.Active()
}

func (c *UeConn) DeferRelease(ctx context.Context, cause s1ap.Cause) {
	if c == nil {
		return
	}

	held := cause
	if !c.deferredCause.CompareAndSwap(nil, &held) {
		return
	}

	link := trace.SpanContextFromContext(ctx)

	c.deferGuard.ArmOnce(deferredReleaseTimeout, func() {
		guardCtx, span := guardSpan(link, "mme/deferred_release_expire", "deferred UE Context Release", 0)
		defer span.End()

		c.Log(guardCtx).Warn("deferred UE Context Release deadline reached; releasing the S1 connection",
			logger.Cause(cause.String()))

		c.resumeDeferredRelease(guardCtx)
	})
}

func (c *UeConn) ResumeDeferredReleaseIfSettled(ctx context.Context) {
	if c == nil || c.deferredCause.Load() == nil {
		return
	}

	if c.MTSignallingPending() {
		return
	}

	c.resumeDeferredRelease(ctx)
}

func (c *UeConn) resumeDeferredRelease(ctx context.Context) {
	cause := c.deferredCause.Swap(nil)
	if cause == nil {
		return
	}

	c.deferGuard.Stop()

	m, ue := c.m, c.UeContext()
	if m == nil || ue == nil {
		return
	}

	logger.From(ctx, logger.MmeLog).Info("resuming the deferred UE Context Release: the pending downlink traffic or signalling has settled")

	m.ReleaseUEContext(ctx, ue, *cause)
}

func (c *UeConn) cancelDeferredRelease() {
	if c == nil {
		return
	}

	c.deferredCause.Store(nil)
	c.deferGuard.Stop()
}
