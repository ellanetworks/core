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
func HandleERABReleaseResponse(m *mme.MME, conn mme.NasWriter, value []byte) {
	msg, err := s1ap.ParseERABReleaseResponse(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode E-RAB Release Response", zap.Error(err))
		return
	}

	ue, ok := resolveUE(m, conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	for _, erab := range msg.ERABReleased {
		logger.MmeLog.Info("E-RAB released at eNB",
			zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)), zap.String("imsi", ue.IMSI()),
			zap.Uint8("e-rab-id", uint8(erab.ERABID)))
	}
}
