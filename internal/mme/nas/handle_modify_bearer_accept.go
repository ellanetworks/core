// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// handleModifyBearerAccept commits the new bearer configuration once the UE accepts
// the in-place modification (TS 24.301 §6.4.2.3). The accept's EPS bearer identity
// selects the PDN connection, so an additional PDN commits to the right bearer.
func handleModifyBearerAccept(m *mme.MME, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	p := m.DefaultPDN(ue)
	if accept, err := eps.ParseModifyEPSBearerContextAccept(plain); err == nil {
		if named := m.LookupPDN(ue, accept.EPSBearerIdentity); named != nil {
			p = named
		}
	}

	if p == nil {
		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	m.StopESMGuard(p)

	if !ue.CommitBearerModification(p) {
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ue.Conn().Log.Info("EPS bearer modified in place", zap.String("imsi", ue.IMSI()), zap.String("apn", p.Apn))

	return nasreply.Handled()
}
