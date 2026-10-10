// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"slices"
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

	if sc.Tunnel.forwardingTEID() == 0 {
		return fmt.Errorf("the UPF allocated no TEID for the forwarding tunnel")
	}

	return nil
}

func (s *SMF) openBearerForwardingTunnel(ctx context.Context, sc *SMContext, ebi uint8, target AnchorBinding) (uint32, error) {
	if ebi == 0 || ebi == sc.EBI {
		if err := s.openForwardingTunnel(ctx, sc, target); err != nil {
			return 0, err
		}

		return sc.Tunnel.forwardingTEID(), nil
	}

	i := slices.IndexFunc(sc.dedicated, func(b *dedicatedBearer) bool { return b.ebi == ebi })
	if i < 0 {
		return 0, fmt.Errorf("no dedicated bearer %d", ebi)
	}

	slot := sc.dedicated[i].slot

	sc.forwardingRelease.Stop()

	if err := s.updateLegLocked(ctx, sc, slot, func(l *bearerLeg) { l.Forwarding = &target }); err != nil {
		return 0, fmt.Errorf("program the bearer's forwarding tunnel: %w", err)
	}

	teid := sc.Tunnel.ChosenTEIDs[chooseIDBearerForwarding(slot)]
	if teid == 0 {
		return 0, fmt.Errorf("the UPF allocated no TEID for the bearer's forwarding tunnel")
	}

	return teid, nil
}

func (d dataPlane) forwards() bool {
	return d.Forwarding != nil || slices.ContainsFunc(d.Bearers, func(l bearerLeg) bool { return l.Forwarding != nil })
}

func (s *SMF) closeForwardingTunnel(ctx context.Context, sc *SMContext) error {
	if sc.Tunnel == nil || !sc.Tunnel.forwards() {
		return nil
	}

	next := sc.Tunnel.dataPlane
	next.Forwarding = nil
	next.Bearers = slices.Clone(next.Bearers)

	for i := range next.Bearers {
		next.Bearers[i].Forwarding = nil
	}

	if err := s.applyDataPlane(ctx, sc, next, sc.policyID()); err != nil {
		return fmt.Errorf("release the forwarding tunnel: %w", err)
	}

	logger.From(ctx, logger.SmfLog).Info("Released an indirect data forwarding tunnel",
		logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID))

	return nil
}

func (s *SMF) scheduleForwardingRelease(ctx context.Context, sc *SMContext) {
	if sc.Tunnel == nil || !sc.Tunnel.forwards() {
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
