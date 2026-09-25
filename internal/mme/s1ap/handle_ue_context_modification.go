// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func handleUEContextModificationResponse(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	resp, err := s1ap.ParseUEContextModificationResponse(value)
	if err != nil {
		handleParseError(ctx, m, radio.Conn, s1ap.ProcUEContextModification, s1ap.TriggeringSuccessfulOutcome, err)
		return
	}

	ue, ueConn, ok := resolveUEIDs(ctx, m, radio.Conn, resp.MMEUES1APID, resp.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, m, radio.Conn, s1ap.ProcUEContextModification, s1ap.TriggeringSuccessfulOutcome, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID()), resp.Diagnostics())

	ue.TouchLastSeen()
}

func handleUEContextModificationFailure(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	failure, err := s1ap.ParseUEContextModificationFailure(value)
	if err != nil {
		handleParseError(ctx, m, radio.Conn, s1ap.ProcUEContextModification, s1ap.TriggeringUnsuccessfulOutcome, err)
		return
	}

	ue, ueConn, ok := resolveUEIDs(ctx, m, radio.Conn, failure.MMEUES1APID, failure.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, m, radio.Conn, s1ap.ProcUEContextModification, s1ap.TriggeringUnsuccessfulOutcome, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID()), failure.Diagnostics())

	ue.TouchLastSeen()

	ueConn.Log(ctx).Warn("eNB refused the UE context modification", logger.Cause(mme.S1apCauseName(failure.Cause)))
}
