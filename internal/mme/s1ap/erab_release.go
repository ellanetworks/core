// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// DeactivateBearer asks the UE to deactivate one EPS bearer, carrying the
// DEACTIVATE EPS BEARER CONTEXT REQUEST with the given ESM cause and procedure
// transaction identity (TS 24.301 §6.4.4).
//
// When the deactivation leaves the UE connected — a PDN disconnect, or a
// reactivation of an additional PDN — the NAS rides in an S1AP E-RAB RELEASE
// COMMAND so the eNB releases that radio bearer in the same step (TS 23.401
// §5.10.3 "Deactivate Bearer Request"). When it instead deactivates the attach
// (first) bearer with reactivation requested, the UE re-attaches and the full UE
// Context Release that follows tears down the radio bearers, so the NAS is sent
// on a Downlink NAS Transport and guarded like other common procedures.
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
