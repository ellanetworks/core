// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// handleERABModifyResponse records the eNB's E-RAB Modify outcome. The procedure
// completes on the NAS Modify Accept, so a failed-to-modify list is logged but
// does not itself abort the modification (TS 36.413 §8.2.2).
func handleERABModifyResponse(m *mme.MME, value []byte) {
	resp, err := s1ap.ParseERABModifyResponse(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode E-RAB Modify Response", zap.Error(err))
		return
	}

	if ue, ok := m.LookupUe(resp.MMEUES1APID); ok {
		ue.TouchLastSeen()
		captureUserLocation(ue, resp.UserLocationInformation)
	}

	if len(resp.ERABFailedToModify) > 0 {
		logger.MmeLog.Warn("eNB failed to modify E-RAB(s)",
			zap.Uint32("mme-ue-id", uint32(resp.MMEUES1APID)), zap.Int("failed", len(resp.ERABFailedToModify)))
	}
}
