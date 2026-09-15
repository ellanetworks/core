// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func (m *MME) OpenForwardingTunnel(ctx context.Context, ue *UeContext, ebi uint8, target models.FTEID) (models.ForwardingTunnel, bool) {
	p := m.LookupPDN(ue, ebi)
	if p == nil {
		return models.ForwardingTunnel{}, false
	}

	ue.forwardingRelease.Stop()

	local, err := m.Session.OpenEPSForwardingTunnel(ctx, p.SessionRef, target)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("could not open an indirect data forwarding tunnel; this E-RAB forwards nothing",
			zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", ebi), zap.Error(err))

		return models.ForwardingTunnel{}, false
	}

	return local, true
}

func (m *MME) CloseForwardingTunnels(ctx context.Context, ue *UeContext) {
	if ue == nil {
		return
	}

	ue.forwardingRelease.Stop()

	for _, p := range m.SnapshotPDNs(ue) {
		if err := m.Session.CloseEPSForwardingTunnel(ctx, p.SessionRef); err != nil {
			logger.From(ctx, logger.MmeLog).Warn("failed to release an indirect data forwarding tunnel",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", p.Ebi), zap.Error(err))
		}
	}
}

var indirectForwardingDuration = 2 * time.Second

func (m *MME) ScheduleForwardingRelease(ctx context.Context, ue *UeContext) {
	if ue == nil {
		return
	}

	link := trace.SpanContextFromContext(ctx)

	ue.forwardingRelease.ArmOnce(indirectForwardingDuration, func() {
		guardCtx, span := guardSpan(link, "mme/forwarding_release_expire", "indirect forwarding", 0)
		defer span.End()

		m.CloseForwardingTunnels(guardCtx, ue)
	})
}
