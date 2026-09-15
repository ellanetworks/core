// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleERABModifyResponse records the eNB's E-RAB Modify outcome. The procedure
// completes on the NAS Modify Accept, so a failed-to-modify list is logged but
// does not itself abort the modification (TS 36.413 §8.2.2).
func handleERABModifyResponse(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	resp, err := s1ap.ParseERABModifyResponse(value)
	if err != nil {
		handleParseError(ctx, m, radio.Conn, s1ap.ProcERABModify, err)
		return
	}

	// Both identities are mandatory but ignore criticality, so an absent one
	// still reaches the handler. resolveUEIDs also rejects a response naming a
	// UE on another radio.
	ue, ueConn, ok := resolveUEIDs(ctx, m, radio.Conn, resp.MMEUES1APID, resp.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, m, radio.Conn, s1ap.ProcERABModify, s1ap.TriggeringSuccessfulOutcome, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID), resp.Diagnostics())

	ue.TouchLastSeen()
	captureUserLocation(ueConn, resp.UserLocationInformation)

	if len(resp.ERABFailedToModify) > 0 {
		logger.From(ctx, logger.MmeLog).Warn("eNB failed to modify E-RAB(s)",
			logger.MMEUeS1apID(uint32(*resp.MMEUES1APID)), zap.Int("failed", len(resp.ERABFailedToModify)))
	}
}
