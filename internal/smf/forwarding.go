// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var indirectForwardingDuration = 2 * time.Second

func (s *SMF) openForwardingTunnel(ctx context.Context, sc *SMContext, target AnchorBinding) error {
	if sc.Tunnel == nil {
		return fmt.Errorf("session %q has no user plane", sc.Ref)
	}

	sc.forwardingRelease.Stop()

	next := sc.Tunnel.dataPlane
	next.Forwarding = &target

	if err := s.applyDataPlane(ctx, sc, next, sc.policyID()); err != nil {
		return fmt.Errorf("program the forwarding tunnel: %w", err)
	}

	if sc.Tunnel.ForwardingTEID == 0 {
		return fmt.Errorf("the UPF allocated no TEID for the forwarding tunnel")
	}

	return nil
}

func (s *SMF) closeForwardingTunnel(ctx context.Context, sc *SMContext) error {
	if sc.Tunnel == nil || sc.Tunnel.Forwarding == nil {
		return nil
	}

	next := sc.Tunnel.dataPlane
	next.Forwarding = nil

	if err := s.applyDataPlane(ctx, sc, next, sc.policyID()); err != nil {
		return fmt.Errorf("release the forwarding tunnel: %w", err)
	}

	sc.Tunnel.ForwardingTEID = 0

	logger.From(ctx, logger.SmfLog).Info("Released an indirect data forwarding tunnel",
		logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID))

	return nil
}

func (s *SMF) scheduleForwardingRelease(ctx context.Context, sc *SMContext) {
	if sc.Tunnel == nil || sc.Tunnel.Forwarding == nil {
		return
	}

	ref := sc.Ref
	link := trace.SpanContextFromContext(ctx)

	sc.forwardingRelease.ArmOnce(indirectForwardingDuration, func() {
		released := s.GetSession(ref)
		if released == nil {
			return
		}

		ctx, span := guardSpan(link, "smf/forwarding_release_expire", "indirect forwarding", 0)
		defer span.End()

		released.Mutex.Lock()
		defer released.Mutex.Unlock()

		if err := s.closeForwardingTunnel(ctx, released); err != nil {
			logger.From(ctx, logger.SmfLog).Warn("failed to release an indirect data forwarding tunnel",
				zap.String("ref", ref), zap.Error(err))
		}
	})
}
