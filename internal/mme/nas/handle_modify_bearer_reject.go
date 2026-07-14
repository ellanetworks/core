// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// handleModifyBearerReject abandons the modification when the UE rejects it
// (TS 24.301 §6.4.2.4), leaving the stored config stale so the backstop retries.
func handleModifyBearerReject(m *mme.MME, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	p := m.DefaultPDN(ue)
	if rej, err := eps.ParseModifyEPSBearerContextReject(plain); err == nil {
		if named := m.LookupPDN(ue, rej.EPSBearerIdentity); named != nil {
			p = named
		}
	}

	if p != nil {
		m.StopESMGuard(p)
		ue.ClearPendingModify(p)
	}

	ue.Conn().Log.Warn("UE rejected EPS bearer modification", zap.String("imsi", ue.IMSI()))

	return nasreply.Handled()
}
