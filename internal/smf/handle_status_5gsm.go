// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

// handle5GSMStatus processes a received 5GSM STATUS (TS 24.501 §6.5.3): every cause
// the clause names aborts the ongoing 5GSM procedure and stops its timer, so that is
// done unconditionally; the local action for any other cause is implementation
// dependent, and abandoning the procedure is the same choice the MME makes for ESM
// (TS 24.301 §6.7). The pending policy is discarded rather than committed, leaving
// PolicyData stale so the reconcile sweep re-derives the modification.
//
// Three cases go on to release the PDU session locally:
//
//   - #43 "invalid PDU session identity": mandated by §6.5.3.
//   - #47 "PTI mismatch" naming the PTI of this session's PDU SESSION ESTABLISHMENT
//     ACCEPT: mandated by §6.5.3. The UE never took the session, so the SMF would
//     otherwise anchor a session no UE has. This is the 5G spelling of the 4G
//     ACTIVATE DEFAULT EPS BEARER CONTEXT REJECT the MME answers by releasing the
//     PDN connection (TS 24.301 §7.3.1 g).
//   - any cause aborting an in-flight release: the user plane is freed when the
//     release starts (TS 23.502 §4.3.4.2 step 2), and no reconcile sweep re-derives
//     a UE-requested release, so the session is removed here or never.
//
// Caller must hold smContext.Mutex.
func (s *SMF) handle5GSMStatus(ctx context.Context, smContext *SMContext, pti, cause uint8) {
	logger.WithTrace(ctx, logger.SmfLog).Warn("N1 Msg 5GSM STATUS received",
		zap.Uint8("pti", pti), zap.Uint8("cause", cause),
		logger.SUPI(smContext.Supi.String()), logger.PDUSessionID(smContext.PDUSessionID))

	smContext.stopProcedureTimer()
	smContext.ClearPTIInUse(pti)
	smContext.pendingPolicy = nil

	establishmentMismatch := cause == nasMessage.Cause5GSMPTIMismatch &&
		smContext.establishmentPTI != 0 && pti == smContext.establishmentPTI

	if cause == nasMessage.Cause5GSMInvalidPDUSessionIdentity || establishmentMismatch || smContext.releasing {
		s.teardownAndRemove(ctx, smContext)
	}
}
