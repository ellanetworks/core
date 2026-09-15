// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

// HandleUEContextReleaseComplete completes the release (TS 36.413): either
// deleting the UE context (detach) or retaining it in ECM-IDLE.
func HandleUEContextReleaseComplete(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseUEContextReleaseComplete(value)
	if err != nil {
		handleParseError(ctx, m, radio.Conn, s1ap.ProcUEContextRelease, err)
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
		logger.From(ctx, logger.MmeLog).Info("UE Context Release Complete for a connection the MME no longer holds",
			logger.MMEUeS1apID(uint32(mmeUEID)), logger.ENBUeS1apID(uint32(enbUEID)))

		return
	}

	reportDiagnostics(ctx, m, radio.Conn, s1ap.ProcUEContextRelease, s1ap.TriggeringSuccessfulOutcome, ueAssociated(ueConn.MMEUES1APID, ueConn.ENBUES1APID()), msg.Diagnostics())

	captureUserLocation(ueConn, msg.UserLocationInformation)

	// Cancel the release-supervision guard so it does not also run the cleanup.
	ueConn.StopReleaseGuard()

	// A UE that is not EMM-REGISTERED (detached, or an aborted in-progress attach) is
	// deleted; a still-registered UE is retained in ECM-IDLE (TS 23.401).
	if ue.EMMState() != mme.EMMRegistered {
		m.DropDeferredServiceRequest(ctx, ue)
		m.ReleaseAllSessions(ctx, ue)
		m.RemoveUe(ue)
		logger.From(ctx, ueConn.Log()).Info("UE context removed", logger.RAT(metrics.RAT4G))

		return
	}

	m.FreeUeConn(ctx, ue)

	// Supervise the UE's reachability while idle: the mobile reachable timer is
	// (re)started when the MME releases the NAS signalling connection (TS 24.301).
	m.StartMobileReachable(ue)

	logger.From(ctx, ueConn.Log()).Info("UE idle", logger.RAT(metrics.RAT4G))

	m.ResumeDeferredServiceRequest(ctx, ue)
}
