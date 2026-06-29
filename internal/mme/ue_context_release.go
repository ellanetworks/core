// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

func (m *MME) ReleaseUEContext(ctx context.Context, ue *UeContext, cause s1ap.Cause) {
	// The idempotency claim is atomic: a NAS guard timeout and an eNB-initiated
	// release request can race to release the same UE from different goroutines. A
	// Release Complete in the gap may already have freed the connection, which is
	// itself a completed release.
	if !m.claimRelease(ue) {
		return
	}

	cmd := &s1ap.UEContextReleaseCommand{
		UES1APIDs: s1ap.UES1APIDs{MMEUES1APID: ue.S1.MMEUES1APID, ENBUES1APID: ue.S1.ENBUES1APID, Pair: true},
		Cause:     cause,
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal UE Context Release Command", zap.Error(err))
		return
	}

	logger.MmeLog.Info("UE Context Release Command", zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)))
	m.SendS1AP(ctx, ue, S1APProcedureUEContextReleaseCommand, b)
}

// handleUEContextReleaseRequest handles an eNB-initiated release (TS 36.413,
// e.g. radio-link failure or inactivity) by issuing a release command.

func (m *MME) releaseUEContextLocally(ue *UeContext, trigger string) {
	registered, imsi, mmeUEID := m.releaseContextLockedPart(ue)

	if !registered {
		m.ReleaseAllSessions(ue)
		logger.MmeLog.Info("aborted incomplete UE registration",
			zap.String("trigger", trigger), zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("imsi", imsi))

		return
	}

	m.DeactivateAllSessions(ue)
	m.StartMobileReachable(ue)
	logger.MmeLog.Info("UE moved to ECM-IDLE",
		zap.String("trigger", trigger), zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("imsi", imsi))
}
