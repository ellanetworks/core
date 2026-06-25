// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleReset processes a RESET from the eNB (TS 36.413 §8.7.1). The eNB has
// lost its UE-associated logical S1 connections and asks the MME to release the
// matching contexts; the MME must answer with RESET ACKNOWLEDGE or the eNB
// cannot reuse the released UE-S1AP-IDs. A whole-interface reset clears every UE
// on the association; a part-of-interface reset clears only the listed ones. The
// SCTP association itself stays up, so the eNB remains S1-Setup-complete.
//
// A released UE that completed registration is kept in ECM-IDLE (the reset
// removed only the radio leg; the EMM registration survives and the UE stays
// pageable); an incomplete attach is aborted. This is the per-UE handling of an
// abrupt radio-context loss, shared with eNB disconnect.
func (m *MME) handleReset(conn nasWriter, value []byte) {
	req, err := s1ap.ParseReset(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Reset", zap.Error(err))
		return
	}

	if req.ResetType.All {
		affected := m.uesOnConn(conn)
		for _, ue := range affected {
			m.releaseUEContextLocally(ue, "S1 reset")
		}

		logger.MmeLog.Info("S1 Reset (whole interface)", zap.Int("released", len(affected)))
		m.sendResetAcknowledge(conn, nil)

		return
	}

	affected := m.uesForConnectionList(conn, req.ResetType.Part)
	for _, ue := range affected {
		m.releaseUEContextLocally(ue, "S1 reset")
	}

	logger.MmeLog.Info("S1 Reset (part of interface)",
		zap.Int("requested", len(req.ResetType.Part)), zap.Int("released", len(affected)))

	// TS 36.413 §8.7.1.2.1: the acknowledge echoes the UE-associated logical
	// S1-connections that were reset.
	m.sendResetAcknowledge(conn, req.ResetType.Part)
}

// uesOnConn returns every UE context bound to the given eNB association.
func (m *MME) uesOnConn(conn nasWriter) []*UeContext {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []*UeContext

	for _, c := range m.conns {
		if c.ue != nil && c.conn == conn {
			out = append(out, c.ue)
		}
	}

	return out
}

// uesForConnectionList resolves the UE contexts named by a part-of-interface
// reset list, scoped to the association the reset arrived on. Each item is
// matched by its MME-UE-S1AP-ID, else by its eNB-UE-S1AP-ID; an item naming no
// known UE is skipped (it is still echoed in the acknowledge).
func (m *MME) uesForConnectionList(conn nasWriter, items []s1ap.UEAssociatedLogicalS1ConnectionItem) []*UeContext {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []*UeContext

	for _, it := range items {
		switch {
		case it.MMEUES1APID != nil:
			if c, ok := m.conns[uint32(*it.MMEUES1APID)]; ok && c.ue != nil && c.conn == conn {
				out = append(out, c.ue)
			}
		case it.ENBUES1APID != nil:
			for _, c := range m.conns {
				if c.ue != nil && c.conn == conn && c.ENBUES1APID == *it.ENBUES1APID {
					out = append(out, c.ue)
					break
				}
			}
		}
	}

	return out
}

// sendResetAcknowledge answers a RESET with RESET ACKNOWLEDGE (TS 36.413
// §9.1.2.7). connectionList is non-nil only for a part-of-interface reset.
func (m *MME) sendResetAcknowledge(conn nasWriter, connectionList []s1ap.UEAssociatedLogicalS1ConnectionItem) {
	ack := &s1ap.ResetAcknowledge{ConnectionList: connectionList}

	b, err := ack.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Reset Acknowledge", zap.Error(err))
		return
	}

	if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamNonUE}); err != nil {
		logger.MmeLog.Error("failed to send Reset Acknowledge", zap.Error(err))
		return
	}

	// Reset handling is not tied to a single UE request span; use a fresh root.
	m.logOutboundS1AP(context.Background(), conn, S1APProcedureResetAcknowledge, b)
}
