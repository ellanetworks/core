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

// ReleaseUEContext starts the S1 UE Context Release procedure for a UE
// (TS 36.413, TS 23.401). It is idempotent per UE so a detach and an
// eNB-initiated release request cannot both emit a command. Whether the context
// is deleted or retained in ECM-IDLE is decided at release-complete from the EMM
// state, since the two state machines are independent.
func handleUEContextReleaseRequest(m *mme.MME, ctx context.Context, conn mme.NasWriter, value []byte) {
	msg, err := s1ap.ParseUEContextReleaseRequest(value)
	if err != nil {
		handleParseError(m, conn, s1ap.ProcUEContextReleaseRequest, err)
		return
	}

	ue, ok := resolveUE(m, conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	// An eNB-initiated release (inactivity/RLF) moves the UE to ECM-IDLE; the EMM
	// context is retained. The cause distinguishes a normal inactivity release
	// from a radio-link failure.
	fields := []zap.Field{
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
		zap.String("imsi", ue.IMSI()),
		zap.String("cause", mme.S1apCauseName(&msg.Cause)),
	}

	// A release that arrives after the NAS security context is established but
	// before the UE is EMM-REGISTERED aborts the attach: the eNB tore down the
	// RRC connection before INITIAL CONTEXT SETUP RESPONSE and ATTACH COMPLETE,
	// so the UE will restart the whole attach. Surface it as a failure rather
	// than a routine idle release, and report whether the eNB acknowledged the
	// context setup (ics-response-received).
	if ue.Secured() && ue.EMMState() == mme.EMMDeregistered {
		icsReceived := false
		if p := m.DefaultPDN(ue); p != nil {
			icsReceived = p.EnbFTEID.TEID != 0
		}

		logger.MmeLog.Warn("UE Context Release Request aborted an in-progress attach",
			append(fields, zap.Bool("ics-response-received", icsReceived))...)
	} else {
		logger.MmeLog.Info("UE Context Release Request", fields...)
	}

	m.ReleaseUEContext(ctx, ue, msg.Cause)
}

// HandleUEContextReleaseComplete completes the release (TS 36.413):
// either deleting the UE context (detach) or retaining it in ECM-IDLE.

func HandleUEContextReleaseComplete(m *mme.MME, conn mme.NasWriter, value []byte) {
	msg, err := s1ap.ParseUEContextReleaseComplete(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode UE Context Release Complete", zap.Error(err))
		return
	}

	// A Release Complete for a detached handover association — the source after
	// notify, or a rejected/superseded target — is identified by its own
	// MME-UE-S1AP-ID and just removes the connection, without touching the UE now
	// active on the other association (TS 36.413 §8.4).
	if m.ReleaseDetachedConn(conn, msg.MMEUES1APID, msg.ENBUES1APID) {
		logger.MmeLog.Info("UE Context Release Complete (handover association)", zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))
		return
	}

	ue, ok := resolveUE(m, conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	// Independent state machines (TS 23.401): a detached UE is deleted; a
	// still-registered UE is retained in ECM-IDLE.
	if ue.EMMState() == mme.EMMDeregistered {
		m.ReleaseAllSessions(ue)
		m.RemoveUe(ue)
		logger.MmeLog.Info("UE context released", zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))

		return
	}

	m.FreeS1Conn(ue)

	// Buffer the downlink bearers so data for the idle UE triggers paging
	// (TS 23.401).
	m.DeactivateAllSessions(ue)

	// Supervise the UE's reachability while idle: the mobile reachable timer is
	// (re)started when the MME releases the NAS signalling connection (TS 24.301).
	m.StartMobileReachable(ue)

	logger.MmeLog.Info("UE moved to ECM-IDLE",
		zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)), zap.String("imsi", ue.IMSI()))
}

// releaseUEContextLocally reclaims a UE whose S1 radio context is gone with no
// per-UE S1AP signalling — an eNB association that dropped abruptly or an S1
// Reset. trigger names the cause for the event log. A UE that completed
// registration is retained in ECM-IDLE under mobile reachable supervision — it
// re-establishes via a Service Request or is implicitly detached if it never
// returns (TS 24.301). A UE that had not completed registration is dropped,
// aborting the procedure.
