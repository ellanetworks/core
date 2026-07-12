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

// handleUplinkLPPaTransport stores an eNB-relayed LPPa PDU on its UE context for
// the LMF to correlate and decode (TS 36.413 §8.14). The PDU is opaque to the
// S1AP layer; the LMF matches it by the E-SMLC-UE-Measurement-ID inside.
func handleUplinkLPPaTransport(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseUplinkUEAssociatedLPPaTransport(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcUplinkUEAssociatedLPPaTransport, err)
		return
	}

	ue, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	ue.SetLPPaMessage([]byte(msg.LPPaPDU))

	logger.From(ctx, radio.Log).Debug("stored uplink LPPa PDU",
		zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)),
		zap.Int("payload-len", len(msg.LPPaPDU)),
	)
}
