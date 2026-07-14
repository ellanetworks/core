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

const esmPTIReserved uint8 = 255

// handleESMStatus aborts the ESM procedure the STATUS names (TS 24.301 §6.7). Cause
// #43 deactivates the bearer, and so does any cause reaching a bearer whose
// deactivation is in flight: the user plane is released when the deactivation starts
// (TS 23.401 §5.4.4) and a UE-requested disconnect leaves no configuration diff for
// the reconcile sweep, so the PDN connection is torn down here or never.
func handleESMStatus(ctx context.Context, m *mme.MME, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	status, err := eps.ParseESMStatus(plain)
	if err != nil {
		return nasreply.Silent(nasreply.ReasonTooShort)
	}

	logger.From(ctx, logger.MmeLog).Warn("received ESM STATUS",
		zap.String("imsi", ue.IMSI()),
		zap.Uint8("ebi", status.EPSBearerIdentity),
		zap.Uint8("pti", status.ProcedureTransactionIdentity),
		zap.Uint8("esm-cause", status.ESMCause))

	// TS 24.301 §7.3.1 f); clause 7 applies ahead of the §6.7 handling below.
	if status.ProcedureTransactionIdentity == esmPTIReserved {
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	// TS 24.301 §7.3.2 g): an EPS bearer identity matching no bearer context is ignored.
	p := m.LookupPDN(ue, status.EPSBearerIdentity)
	if p == nil {
		return nasreply.Silent(nasreply.ReasonNoContext)
	}

	m.StopESMGuard(p)

	if status.ESMCause == esmCauseInvalidEPSBearerIdentity || ue.BearerDeactivating(p) {
		m.DeactivatePDN(ctx, ue, p)

		return nasreply.Handled()
	}

	ue.ClearPendingModify(p)

	return nasreply.Handled()
}
