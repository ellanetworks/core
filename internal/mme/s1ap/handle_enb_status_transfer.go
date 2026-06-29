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

// handleENBStatusTransfer relays the source's status container to the target as an
// MME STATUS TRANSFER (TS 36.413 §8.4.6/§8.4.7). Optional: the source may omit it,
// so it never gates completion. conn is the source.
func handleENBStatusTransfer(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	st, err := s1ap.ParseENBStatusTransfer(value)
	if err != nil {
		handleParseError(m, conn, s1ap.ProcENBStatusTransfer, err)
		return
	}

	ue, ok := resolveUE(m, conn, st.MMEUES1APID, st.ENBUES1APID)
	if !ok {
		return
	}

	targetConn, targetMMEID, targetENBID, ok := m.HandoverStatusTarget(ue)
	if !ok {
		logger.MmeLog.Warn("eNB Status Transfer with no handover in progress", zap.Uint32("mme-ue-id", uint32(st.MMEUES1APID)))

		return
	}

	mst := &s1ap.MMEStatusTransfer{MMEUES1APID: targetMMEID, ENBUES1APID: targetENBID, Container: st.Container}

	b, err := mst.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal MME Status Transfer", zap.Error(err))
		return
	}

	m.SendS1APConn(ctx, targetConn, mme.S1APProcedureMMEStatusTransfer, b)
}
