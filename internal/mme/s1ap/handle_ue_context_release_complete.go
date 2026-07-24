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
		logger.MmeLog.Warn("failed to decode UE Context Release Complete", zap.Error(err))
		return
	}

	// A Release Complete for a detached association — a handover source after notify, a
	// rejected/superseded target, or a connection superseded by a UE re-establishing on
	// a new one — removes only that connection, leaving the UE active on its current
	// association (TS 36.413 §8.3, §8.4).
	if m.ReleaseDetachedConn(radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID) {
		logger.MmeLog.Info("UE Context Release Complete (detached association)", zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))
		return
	}

	ue, ok := resolveUE(m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	captureUserLocation(ue, msg.UserLocationInformation)

	ue.TouchLastSeen()

	// Cancel the release-supervision guard so it does not also run the cleanup.
	ue.Conn().StopReleaseGuard()

	// A UE that is not EMM-REGISTERED (detached, or an aborted in-progress attach) is
	// deleted; a still-registered UE is retained in ECM-IDLE (TS 23.401).
	if ue.EMMState() != mme.EMMRegistered {
		m.ReleaseAllSessions(ctx, ue)
		m.RemoveUe(ue)
		logger.MmeLog.Info("UE context released", zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))

		return
	}

	m.FreeUeConn(ue)

	// Supervise the UE's reachability while idle: the mobile reachable timer is
	// (re)started when the MME releases the NAS signalling connection (TS 24.301).
	m.StartMobileReachable(ue)

	logger.MmeLog.Info("UE moved to ECM-IDLE",
		zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)), zap.String("imsi", ue.IMSI()))
}
