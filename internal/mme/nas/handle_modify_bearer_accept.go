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

func handleModifyBearerAccept(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn, accept *eps.ModifyEPSBearerContextAccept) nasreply.Disposition {
	p := m.LookupPDN(ue, uint8(accept.EPSBearerIdentity))

	if p == nil {
		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	m.StopESMGuard(p)

	if cause, ok := fiveGSMCauseFromPCOs(accept.ProtocolConfigurationOptions, accept.ExtendedProtocolConfigurationOptions); ok {
		ueConn.Log(ctx).Warn("UE discarded the mapped 5GS QoS parameters of the bearer modification", zap.String("apn", p.Apn), logger.FiveGSMCause(cause))
	}

	if !ue.CommitBearerModification(p) {
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ueConn.Log(ctx).Info("EPS bearer modified in place", zap.String("apn", p.Apn))

	return nasreply.Handled()
}
