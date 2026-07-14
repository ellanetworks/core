// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// handleDeactivateBearerAccept finalises an EPS bearer deactivation (TS 24.301 §6.4.4.3).
func handleDeactivateBearerAccept(ctx context.Context, m *mme.MME, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	p := m.DefaultPDN(ue)
	if accept, err := eps.ParseDeactivateEPSBearerContextAccept(plain); err == nil {
		if named := m.LookupPDN(ue, accept.EPSBearerIdentity); named != nil {
			p = named
		}
	}

	if p == nil {
		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	m.StopESMGuard(p)

	if ue.BearerReleaseOnly(p) {
		logger.From(ctx, logger.MmeLog).Info("PDN connection released", zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn))
	} else {
		logger.From(ctx, logger.MmeLog).Info("EPS bearer deactivated for reactivation; UE will re-attach", zap.String("imsi", ue.IMSI()))
	}

	m.DeactivatePDN(ctx, ue, p)

	return nasreply.Handled()
}
