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

// HandleUEContextReleaseComplete completes the release (TS 36.413): either
// deleting the UE context (detach) or retaining it in ECM-IDLE.
func HandleUEContextReleaseComplete(m *mme.MME, ctx context.Context, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseUEContextReleaseComplete(value)
	if err != nil {
		handleParseError(m, radio.Conn, s1ap.ProcUEContextRelease, err)
		return
	}

	if msg.MMEUES1APID == nil || msg.ENBUES1APID == nil {
		logger.MmeLog.Warn("UE Context Release Complete without both UE S1AP IDs")
		sendErrorIndication(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID, causeMissingUES1APID)

		return
	}

	mmeUEID, enbUEID := *msg.MMEUES1APID, *msg.ENBUES1APID

	// A Release Complete for a detached association removes only that connection; the UE
	// stays active on its current association (TS 36.413 §8.3, §8.4).
	if m.ReleaseDetachedConn(radio.Conn, mmeUEID, enbUEID) {
		logger.MmeLog.Info("UE Context Release Complete (detached association)", zap.Uint32("mme-ue-id", uint32(mmeUEID)))
		return
	}

	ue, ueConn, ok := resolveUEQuiet(m, radio.Conn, mmeUEID, enbUEID)
	if !ok {
		logger.MmeLog.Info("UE Context Release Complete for a connection the MME no longer holds",
			zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.Uint32("enb-ue-id", uint32(enbUEID)))

		return
	}

	reportDiagnostics(m, ctx, radio.Conn, s1ap.ProcUEContextRelease, s1ap.TriggeringSuccessfulOutcome, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID), msg.Diagnostics())

	captureUserLocation(ueConn, msg.UserLocationInformation)

	ue.TouchLastSeen()

	// Cancel the release-supervision guard so it does not also run the cleanup.
	ueConn.StopReleaseGuard()

	// A UE that is not EMM-REGISTERED (detached, or an aborted in-progress attach) is
	// deleted; a still-registered UE is retained in ECM-IDLE (TS 23.401).
	if ue.EMMState() != mme.EMMRegistered {
		m.ReleaseAllSessions(ctx, ue)
		m.RemoveUe(ue)
		logger.MmeLog.Info("UE context released", zap.Uint32("mme-ue-id", uint32(mmeUEID)))

		return
	}

	m.FreeUeConn(ue)

	// Supervise the UE's reachability while idle: the mobile reachable timer is
	// (re)started when the MME releases the NAS signalling connection (TS 24.301).
	m.StartMobileReachable(ue)

	logger.MmeLog.Info("UE moved to ECM-IDLE",
		zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("imsi", ue.IMSI()))
}
