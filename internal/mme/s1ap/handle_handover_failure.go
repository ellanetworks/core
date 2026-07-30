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
// The failure carries the target's MME-UE-S1AP-ID.
func handleHandoverFailure(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	fail, err := s1ap.ParseHandoverFailure(value)
	if err != nil {
		logger.From(ctx, logger.MmeLog).Warn("failed to decode Handover Failure", zap.Error(err))
		return
	}

	if fail.MMEUES1APID == nil {
		logger.From(ctx, logger.MmeLog).Warn("Handover Failure without an MME-UE-S1AP-ID")
		sendErrorIndication(m, radio.Conn, nil, nil, causeMissingUES1APID)

		return
	}

	ue, ok := m.LookupUe(*fail.MMEUES1APID)
	if !ok {
		return
	}

	ue.TouchLastSeen()

	if !m.HandoverTargetMatches(ue, *fail.MMEUES1APID, radio.Conn) {
		return
	}

	// Relay the target's cause in the HANDOVER PREPARATION FAILURE to the source; the
	// spec asks for "an appropriate cause value" (TS 36.413 §8.4.1.3), and the target's
	// reason is the most informative. An omitted Cause still fails the preparation, so
	// the source is not left waiting (§10.3.5).
	cause := causeHandoverPrepUnspecific
	if fail.Cause != nil {
		cause = *fail.Cause
	}

	logger.From(ctx, logger.MmeLog).Info("Handover Failure",
		zap.Uint32("target-mme-ue-id", uint32(*fail.MMEUES1APID)),
		zap.String("cause", mme.S1apCauseName(&cause)))

	m.FailHandoverToSource(ctx, ue, cause)
}
