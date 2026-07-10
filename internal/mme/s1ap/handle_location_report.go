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

// handleLocationReport records the UE's serving cell from an eNB LOCATION REPORT
// (TS 36.413 §8.12).
func handleLocationReport(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseLocationReport(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcLocationReport, err)
		return
	}

	ue, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	ue.Conn().UpdateLocation(msg.EUTRANCGI, msg.TAI)

	logger.From(ctx, radio.Log).Debug("Location Report",
		zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)),
		zap.Int("event-type", int(msg.RequestType.EventType)),
	)
}
