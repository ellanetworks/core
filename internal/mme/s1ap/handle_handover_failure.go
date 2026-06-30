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

// handleHandoverFailure fails the preparation toward the source when the target
// could not admit the handover, leaving the UE on the source (TS 36.413 §8.4.2.3).
// conn is the target; the failure carries the target's MME-UE-S1AP-ID.
func handleHandoverFailure(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	fail, err := s1ap.ParseHandoverFailure(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Failure", zap.Error(err))
		return
	}

	ue, ok := m.LookupUe(fail.MMEUES1APID)
	if !ok {
		return
	}

	if !m.HandoverTargetMatches(ue, fail.MMEUES1APID, conn) {
		return
	}

	logger.MmeLog.Info("Handover Failure", zap.Uint32("target-mme-ue-id", uint32(fail.MMEUES1APID)))
	m.FailHandoverToSource(ctx, ue, causeHOFailureInTarget)
}
