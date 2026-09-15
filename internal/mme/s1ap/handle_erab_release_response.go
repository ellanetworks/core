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

// HandleERABReleaseResponse logs the eNB's confirmation that each E-RAB was
// released (TS 36.413 §8.2.3).
func HandleERABReleaseResponse(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseERABReleaseResponse(value)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode E-RAB Release Response", zap.Error(err))
		return
	}

	ue, ueConn, ok := resolveUEIDs(ctx, m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, m, radio.Conn, s1ap.ProcERABRelease, s1ap.TriggeringSuccessfulOutcome, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID()), msg.Diagnostics())

	ue.TouchLastSeen()
	captureUserLocation(ueConn, msg.UserLocationInformation)

	for _, erab := range msg.ERABReleased {
		ueConn.Log(ctx).Info("E-RAB released at eNB",
			logger.SUPI(ue.Supi().String()),
			logger.ERABID(uint8(erab.ERABID)))
	}
}
