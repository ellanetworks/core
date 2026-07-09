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
// leaving the UE on the source (TS 36.413 §8.4.5).
func handleHandoverCancel(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	cancel, err := s1ap.ParseHandoverCancel(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcHandoverCancel, err)
		return
	}

	ue, ok := resolveUE(m, radio.Conn, cancel.MMEUES1APID, cancel.ENBUES1APID)
	if !ok {
		return
	}

	ue.TouchLastSeen()

	// Relay the source's HANDOVER CANCEL Cause to the target when releasing its
	// prepared resources (TS 36.413 §8.4.5).
	if releaseConn, releaseMMEID, releaseENBID, pair, has := m.CancelHandover(ue); has {
		mme.SendUEContextRelease(ctx, m, releaseConn, releaseMMEID, releaseENBID, pair, cancel.Cause)
	}

	ack := &s1ap.HandoverCancelAcknowledge{MMEUES1APID: cancel.MMEUES1APID, ENBUES1APID: cancel.ENBUES1APID}

	b, err := ack.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal Handover Cancel Acknowledge", zap.Error(err))
		return
	}

	logger.From(ctx, logger.MmeLog).Info("Handover Cancel", zap.Uint32("mme-ue-id", uint32(cancel.MMEUES1APID)))
	m.SendS1APConn(ctx, radio.Conn, mme.S1APProcedureHandoverCancelAcknowledge, b)
}
