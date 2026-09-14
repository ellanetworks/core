// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

func (m *MME) OpenForwardingTunnel(ctx context.Context, ue *UeContext, ebi uint8, target models.FTEID) (models.ForwardingTunnel, bool) {
	p := m.LookupPDN(ue, ebi)
	if p == nil {
		return models.ForwardingTunnel{}, false
	}

	local, err := m.Session.OpenEPSForwardingTunnel(ctx, p.SessionRef, target)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("could not open an indirect data forwarding tunnel; this E-RAB forwards nothing",
			zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", ebi), zap.Error(err))

		return models.ForwardingTunnel{}, false
	}

	return local, true
}

func (m *MME) CloseForwardingTunnels(ctx context.Context, ue *UeContext) {
	for _, p := range m.SnapshotPDNs(ue) {
		if err := m.Session.CloseEPSForwardingTunnel(ctx, p.SessionRef); err != nil {
			logger.From(ctx, logger.MmeLog).Warn("failed to release an indirect data forwarding tunnel",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", p.Ebi), zap.Error(err))
		}
	}
}

func (m *MME) ScheduleForwardingRelease(ue *UeContext) {
	for _, p := range m.SnapshotPDNs(ue) {
		m.Session.ScheduleEPSForwardingRelease(p.SessionRef)
	}
}
