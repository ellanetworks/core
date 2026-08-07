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

// captureUserLocation records the serving cell from an optional S1AP User
// Location Information IE (TS 36.413 §9.2.1.93) that an eNB may attach to E-RAB
// and release messages. It is a no-op when the IE is absent.
func captureUserLocation(ueConn *mme.UeConn, uli *s1ap.UserLocationInformation) {
	if uli == nil {
		return
	}

	ueConn.UpdateLocation(uli.EUTRANCGI, uli.TAI)
}

// handleLocationReport records the UE's serving cell from an eNB LOCATION REPORT
// (TS 36.413 §8.12).
func handleLocationReport(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseLocationReport(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcLocationReport, err)
		return
	}

	_, ueConn, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(m, ctx, radio.Conn, s1ap.ProcLocationReport, s1ap.TriggeringInitiatingMessage, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID), msg.Diagnostics())

	if msg.EUTRANCGI != nil && msg.TAI != nil {
		ueConn.UpdateLocation(*msg.EUTRANCGI, *msg.TAI)
	}

	fields := []zap.Field{zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID))}
	if msg.RequestType != nil {
		fields = append(fields, zap.Int("event-type", int(msg.RequestType.EventType)))
	}

	logger.From(ctx, radio.Log).Debug("Location Report", fields...)
}
