// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

// handle5GSMStatus aborts the 5GSM procedure the STATUS names. The #43 and #47
// releases are mandated by TS 24.501 §6.5.3; aborting an in-flight release also
// releases, because the user plane is freed when the release starts (TS 23.502
// §4.3.4.2 step 2) and no sweep re-derives a UE-requested release.
//
// Caller must hold smContext.Mutex.
func (s *SMF) handle5GSMStatus(ctx context.Context, smContext *SMContext, pti, cause uint8) {
	logger.WithTrace(ctx, logger.SmfLog).Warn("N1 Msg 5GSM STATUS received",
		zap.Uint8("pti", pti), zap.Uint8("cause", cause),
		logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	smContext.stopProcedureTimer()
	smContext.ClearPTIInUse(pti)
	smContext.pendingPolicy = nil

	establishmentMismatch := cause == fgs.GSMCausePTIMismatch &&
		smContext.establishmentPTI != 0 && pti == smContext.establishmentPTI

	if cause == fgs.GSMCauseInvalidPDUSessionIdentity || establishmentMismatch || smContext.releasing {
		s.teardownAndRemove(ctx, smContext)
	}
}
