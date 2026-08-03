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

// handleReset releases the UE contexts a RESET names and answers with RESET
// ACKNOWLEDGE, which the eNB needs before it can reuse the released UE-S1AP-IDs
// (TS 36.413 §8.7.1). A whole-interface reset clears every UE on the association, a
// part-of-interface reset only the listed ones. The SCTP association stays up.
func handleReset(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	req, err := s1ap.ParseReset(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcReset, err)
		return
	}

	cause := "(none)"
	if req.Cause != nil {
		cause = mme.S1apCauseName(req.Cause)
	}

	if req.ResetType.All {
		affected := m.ConnsOnConn(radio.Conn)
		m.ReclaimConns(affected, "S1 reset")

		logger.From(ctx, radio.Log).Info("S1 Reset (whole interface)",
			zap.String("cause", cause), zap.Int("connections", len(affected)))
		sendResetAcknowledge(m, ctx, radio.Conn, nil, req.Diagnostics())

		return
	}

	affected := m.ConnsForConnectionList(radio.Conn, req.ResetType.Part)
	m.ReclaimConns(affected, "S1 reset")

	logger.From(ctx, radio.Log).Info("S1 Reset (part of interface)",
		zap.String("cause", cause),
		zap.Int("requested", len(req.ResetType.Part)),
		zap.Int("connections", len(affected)))

	// TS 36.413 §8.7.1.2.1: the acknowledge echoes the UE-associated logical
	// S1-connections that were reset.
	sendResetAcknowledge(m, ctx, radio.Conn, req.ResetType.Part, req.Diagnostics())
}

// sendResetAcknowledge answers a RESET with RESET ACKNOWLEDGE (TS 36.413
// §9.1.2.7). connectionList is non-nil only for a part-of-interface reset.
func sendResetAcknowledge(m *mme.MME, ctx context.Context, conn mme.S1APWriter, connectionList []s1ap.UEAssociatedLogicalS1ConnectionItem, diag s1ap.Diagnostics) {
	ack := &s1ap.ResetAcknowledge{ConnectionList: connectionList}

	// §10.3.4.2 reports in the response message of the procedure where it has one.
	if diag.ReportRequired() {
		ack.CriticalityDiagnostics = &s1ap.CriticalityDiagnostics{
			ProcedureCriticality:      s1ap.Ptr(s1ap.ProcedureCriticality(s1ap.ProcReset)),
			IEsCriticalityDiagnostics: diag.Report(),
		}
	}

	b, err := ack.Marshal()
	if err != nil {
		m.RadioLog(conn).Error("failed to marshal Reset Acknowledge", zap.Error(err))
		return
	}

	m.SendToRadio(ctx, conn, mme.S1APProcedureResetAcknowledge, b)
}
