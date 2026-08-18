// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/s1ap"
)

func (m *MME) releaseTo5GS(ctx context.Context, ue *UeContext, cause s1ap.Cause) {
	m.ReleaseAllSessions(ctx, ue)
	ue.TransitionTo(EMMDeregistered)
	m.ReleaseUEContext(ctx, ue, cause)
}

func (m *MME) CancelRegistration(ctx context.Context, supi etsi.SUPI) {
	ue, ok := m.LookupUeBySupi(supi)
	if !ok || ue.EMMState() != EMMRegistered {
		return
	}

	if _, relocating := m.RelocationToFiveGS(ue); relocating {
		return
	}

	if ue.IdleMobilityTo5GSPending() {
		return
	}

	m.releaseTo5GS(ctx, ue, CauseNASNormalRelease)

	logger.From(ctx, logger.MmeLog).Info("UE registered in 5GS; dropping its EPS registration and remaining PDN connections",
		logger.SUPI(supi.String()))
}
