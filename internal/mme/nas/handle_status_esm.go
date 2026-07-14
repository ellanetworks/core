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

// esmPTIReserved is the reserved procedure transaction identity (TS 24.301 §9.4).
const esmPTIReserved uint8 = 255

// handleESMStatus processes a received ESM STATUS (TS 24.301 §6.7): it aborts the
// ongoing ESM procedure for the named bearer and stops its guard. ESM cause #43
// "invalid EPS bearer identity" deactivates the bearer locally, as does any cause
// reaching a bearer whose deactivation is in flight: the user plane is released
// when the deactivation starts (TS 23.401 §5.4.4), and a UE-requested disconnect
// leaves no configuration diff for the reconcile sweep to re-derive, so the PDN
// connection has to be torn down here or never. An ESM STATUS is never answered
// with another STATUS.
//
// Cause #81 "invalid PTI value" aborts by EPS bearer identity rather than by PTI:
// the MME's network-initiated requests carry an unassigned PTI and an assigned EPS
// bearer identity, so the UE's STATUS names the bearer.
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

	// TS 24.301 §7.3.1 f): a reserved PTI names no transaction, so the message is
	// ignored. Clause 7 applies in order of precedence, ahead of §6.7.
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
