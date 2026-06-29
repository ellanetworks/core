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

// handleHandoverCancel releases any prepared target resources and acknowledges,
// leaving the UE on the source (TS 36.413 §8.4.5). conn is the source.
func handleHandoverCancel(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	cancel, err := s1ap.ParseHandoverCancel(value)
	if err != nil {
		handleParseError(m, conn, s1ap.ProcHandoverCancel, err)
		return
	}

	ue, ok := resolveUE(m, conn, cancel.MMEUES1APID, cancel.ENBUES1APID)
	if !ok {
		return
	}

	if releaseConn, releaseMMEID, releaseENBID, has := m.CancelHandover(ue); has {
		mme.SendUEContextRelease(m, ctx, releaseConn, releaseMMEID, releaseENBID)
	}

	ack := &s1ap.HandoverCancelAcknowledge{MMEUES1APID: cancel.MMEUES1APID, ENBUES1APID: cancel.ENBUES1APID}

	b, err := ack.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Handover Cancel Acknowledge", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Handover Cancel", zap.Uint32("mme-ue-id", uint32(cancel.MMEUES1APID)))
	m.SendS1APConn(ctx, conn, mme.S1APProcedureHandoverCancelAcknowledge, b)
}
