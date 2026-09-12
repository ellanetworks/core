// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

const deferredReleaseTimeout = 30 * time.Second

func (c *UeConn) MTSignallingPending() bool {
	if c == nil {
		return false
	}

	if c.UeContext().MTDeliveryInProgress() {
		return true
	}

	return c.nasGuard.Active() || c.esmInfoGuard.Active()
}

func (c *UeConn) DeferRelease(cause s1ap.Cause) {
	if c == nil {
		return
	}

	held := cause
	c.deferredCause.Store(&held)

	c.deferGuard.ArmOnce(deferredReleaseTimeout, func() {
		logger.From(context.Background(), c.Log()).Warn("deferred UE Context Release deadline reached; releasing the S1 connection",
			zap.String("cause", cause.String()))

		c.resumeDeferredRelease(context.Background())
	})
}

func (c *UeConn) ResumeDeferredReleaseIfSettled() {
	if c == nil || c.deferredCause.Load() == nil {
		return
	}

	if c.MTSignallingPending() {
		return
	}

	c.resumeDeferredRelease(context.Background())
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
