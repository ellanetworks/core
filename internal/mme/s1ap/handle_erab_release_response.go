// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// HandleERABReleaseResponse logs the eNB's confirmation that each E-RAB was
// released (TS 36.413 §8.2.3).
func HandleERABReleaseResponse(m *mme.MME, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseERABReleaseResponse(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode E-RAB Release Response", zap.Error(err))
		return
	}

	ue, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	ue.TouchLastSeen()
	captureUserLocation(ue, msg.UserLocationInformation)

	for _, erab := range msg.ERABReleased {
		ue.Conn().Log.Info("E-RAB released at eNB",
			zap.String("imsi", ue.IMSI()),
			zap.Uint8("e-rab-id", uint8(erab.ERABID)))
	}
}
