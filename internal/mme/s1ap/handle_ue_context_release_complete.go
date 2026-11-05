// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

// HandleUEContextReleaseComplete completes the release (TS 36.413): either
// deleting the UE context (detach) or retaining it in ECM-IDLE.
func HandleUEContextReleaseComplete(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseUEContextReleaseComplete(value)
	if err != nil {
		handleParseError(ctx, m, radio.Conn, s1ap.ProcUEContextRelease, s1ap.TriggeringSuccessfulOutcome, err)
		return
	}

	if msg.MMEUES1APID == nil || msg.ENBUES1APID == nil {
		logger.From(ctx, logger.MmeLog).Warn("UE Context Release Complete without both UE S1AP IDs")
		sendErrorIndication(ctx, m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID, causeMissingUES1APID)

		return
	}

	mmeUEID, enbUEID := *msg.MMEUES1APID, *msg.ENBUES1APID

	// A Release Complete for a detached association removes only that connection; the UE
	// stays active on its current association (TS 36.413 §8.3, §8.4).
	if m.ReleaseDetachedConn(radio.Conn, mmeUEID, enbUEID) {
		logger.From(ctx, logger.MmeLog).Debug("UE Context Release Complete (detached association)", logger.MMEUeS1apID(uint32(mmeUEID)))
		return
	}

	ue, ueConn, ok := resolveUEQuiet(m, radio.Conn, mmeUEID, enbUEID)
	if !ok {
		logger.From(ctx, logger.MmeLog).Info("UE Context Release Complete for a connection the MME does not hold",
			logger.MMEUeS1apID(uint32(mmeUEID)), logger.ENBUeS1apID(uint32(enbUEID)))
		m.ReleaseConnsForInconsistentPair(ctx, radio.Conn, mmeUEID, enbUEID, "UE Context Release Complete with an inconsistent S1AP ID pair")

		return
	}

	reportDiagnostics(ctx, m, radio.Conn, s1ap.ProcUEContextRelease, s1ap.TriggeringSuccessfulOutcome, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID()), msg.Diagnostics())

	captureUserLocation(ueConn, msg.UserLocationInformation)

	m.CompleteUEContextRelease(ctx, ue, ueConn)
}
