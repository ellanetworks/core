// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// releaseUEContext starts the S1 UE Context Release procedure for a UE
// (TS 36.413, TS 23.401). It is idempotent per UE so a detach and an
// eNB-initiated release request cannot both emit a command. Whether the context
// is deleted or retained in ECM-IDLE is decided at release-complete from the EMM
// state, since the two state machines are independent.
func (m *MME) releaseUEContext(ctx context.Context, ue *UeContext, cause s1ap.Cause) {
	// The idempotency claim is atomic: a NAS guard timeout and an eNB-initiated
	// release request can race to release the same UE from different goroutines. A
	// Release Complete in the gap may already have freed the connection, which is
	// itself a completed release.
	if !m.claimRelease(ue) {
		return
	}

	cmd := &s1ap.UEContextReleaseCommand{
		UES1APIDs: s1ap.UES1APIDs{MMEUES1APID: ue.s1.MMEUES1APID, ENBUES1APID: ue.s1.ENBUES1APID, Pair: true},
		Cause:     cause,
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal UE Context Release Command", zap.Error(err))
		return
	}

	logger.MmeLog.Info("UE Context Release Command", zap.Uint32("mme-ue-id", uint32(ue.s1.MMEUES1APID)))
	m.sendS1AP(ctx, ue, S1APProcedureUEContextReleaseCommand, b)
}

// handleUEContextReleaseRequest handles an eNB-initiated release (TS 36.413,
// e.g. radio-link failure or inactivity) by issuing a release command.
func (m *MME) handleUEContextReleaseRequest(ctx context.Context, conn nasWriter, value []byte) {
	msg, err := s1ap.ParseUEContextReleaseRequest(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode UE Context Release Request", zap.Error(err))
		return
	}

	ue, ok := m.resolveUE(conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	// An eNB-initiated release (inactivity/RLF) moves the UE to ECM-IDLE; the EMM
	// context is retained. The cause distinguishes a normal inactivity release
	// from a radio-link failure.
	fields := []zap.Field{
		zap.Uint32("mme-ue-id", uint32(ue.s1.MMEUES1APID)),
		zap.String("imsi", ue.imsi),
		zap.String("cause", s1apCauseName(&msg.Cause)),
	}

	// A release that arrives after the NAS security context is established but
	// before the UE is EMM-REGISTERED aborts the attach: the eNB tore down the
	// RRC connection before INITIAL CONTEXT SETUP RESPONSE and ATTACH COMPLETE,
	// so the UE will restart the whole attach. Surface it as a failure rather
	// than a routine idle release, and report whether the eNB acknowledged the
	// context setup (ics-response-received).
	if ue.secured && ue.emmState.load() == EMMDeregistered {
		icsReceived := false
		if p := m.defaultPDN(ue); p != nil {
			icsReceived = p.enbFTEID.TEID != 0
		}

		logger.MmeLog.Warn("UE Context Release Request aborted an in-progress attach",
			append(fields, zap.Bool("ics-response-received", icsReceived))...)
	} else {
		logger.MmeLog.Info("UE Context Release Request", fields...)
	}

	m.releaseUEContext(ctx, ue, msg.Cause)
}

// handleUEContextReleaseComplete completes the release (TS 36.413):
// either deleting the UE context (detach) or retaining it in ECM-IDLE.
func (m *MME) handleUEContextReleaseComplete(conn nasWriter, value []byte) {
	msg, err := s1ap.ParseUEContextReleaseComplete(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode UE Context Release Complete", zap.Error(err))
		return
	}

	// A Release Complete for a detached handover association — the source after
	// notify, or a rejected/superseded target — is identified by its own
	// MME-UE-S1AP-ID and just removes the connection, without touching the UE now
	// active on the other association (TS 36.413 §8.4).
	if m.releaseDetachedConn(conn, msg.MMEUES1APID, msg.ENBUES1APID) {
		logger.MmeLog.Info("UE Context Release Complete (handover association)", zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))
		return
	}

	ue, ok := m.resolveUE(conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	// Independent state machines (TS 23.401): a detached UE is deleted; a
	// still-registered UE is retained in ECM-IDLE.
	if ue.emmState.load() == EMMDeregistered {
		m.releaseAllSessions(ue)
		m.removeUe(ue)
		logger.MmeLog.Info("UE context released", zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))

		return
	}

	m.freeS1Conn(ue)

	// Buffer the downlink bearers so data for the idle UE triggers paging
	// (TS 23.401).
	m.deactivateAllSessions(ue)

	// Supervise the UE's reachability while idle: the mobile reachable timer is
	// (re)started when the MME releases the NAS signalling connection (TS 24.301).
	m.startMobileReachable(ue)

	logger.MmeLog.Info("UE moved to ECM-IDLE",
		zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)), zap.String("imsi", ue.imsi))
}

// releaseUEContextLocally reclaims a UE whose S1 radio context is gone with no
// per-UE S1AP signalling — an eNB association that dropped abruptly or an S1
// Reset. trigger names the cause for the event log. A UE that completed
// registration is retained in ECM-IDLE under mobile reachable supervision — it
// re-establishes via a Service Request or is implicitly detached if it never
// returns (TS 24.301). A UE that had not completed registration is dropped,
// aborting the procedure.
func (m *MME) releaseUEContextLocally(ue *UeContext, trigger string) {
	registered, imsi, mmeUEID := m.releaseContextLockedPart(ue)

	if !registered {
		m.releaseAllSessions(ue)
		logger.MmeLog.Info("aborted incomplete UE registration",
			zap.String("trigger", trigger), zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("imsi", imsi))

		return
	}

	m.deactivateAllSessions(ue)
	m.startMobileReachable(ue)
	logger.MmeLog.Info("UE moved to ECM-IDLE",
		zap.String("trigger", trigger), zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("imsi", imsi))
}
