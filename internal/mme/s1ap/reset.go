// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleReset releases the UE contexts a RESET names and answers with RESET
// ACKNOWLEDGE, which the eNB needs before it can reuse the released UE-S1AP-IDs
// (TS 36.413 §8.7.1). A whole-interface reset clears every UE on the association,
// a part-of-interface reset only the listed ones; the SCTP association stays up,
// so the eNB remains S1-Setup-complete. A released UE that completed registration
// is kept pageable in ECM-IDLE; an incomplete attach is aborted.
func handleReset(m *mme.MME, conn mme.NasWriter, value []byte) {
	req, err := s1ap.ParseReset(value)
	if err != nil {
		handleParseError(m, conn, s1ap.ProcReset, err)
		return
	}

	if req.ResetType.All {
		affected := m.ConnsOnConn(conn)
		m.ReclaimConns(affected, "S1 reset")

		logger.MmeLog.Info("S1 Reset (whole interface)", zap.Int("connections", len(affected)))
		sendResetAcknowledge(m, conn, nil)

		return
	}

	affected := m.ConnsForConnectionList(conn, req.ResetType.Part)
	m.ReclaimConns(affected, "S1 reset")

	logger.MmeLog.Info("S1 Reset (part of interface)",
		zap.Int("requested", len(req.ResetType.Part)), zap.Int("connections", len(affected)))

	// TS 36.413 §8.7.1.2.1: the acknowledge echoes the UE-associated logical
	// S1-connections that were reset.
	sendResetAcknowledge(m, conn, req.ResetType.Part)
}

// sendResetAcknowledge answers a RESET with RESET ACKNOWLEDGE (TS 36.413
// §9.1.2.7). connectionList is non-nil only for a part-of-interface reset.
func sendResetAcknowledge(m *mme.MME, conn mme.NasWriter, connectionList []s1ap.UEAssociatedLogicalS1ConnectionItem) {
	ack := &s1ap.ResetAcknowledge{ConnectionList: connectionList}

	b, err := ack.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Reset Acknowledge", zap.Error(err))
		return
	}

	if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: mme.S1apWirePPID, Stream: mme.S1apStreamNonUE}); err != nil {
		logger.MmeLog.Error("failed to send Reset Acknowledge", zap.Error(err))
		return
	}

	// Reset handling is not tied to a single UE request span; use a fresh root.
	m.LogOutboundS1AP(context.Background(), conn, mme.S1APProcedureResetAcknowledge, b)
}
