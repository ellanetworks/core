// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// detachTypeReattachNotRequired is the network-originating detach type meaning
// the UE shall not re-attach (TS 24.301) — used when a subscriber is
// removed.
const detachTypeReattachNotRequired uint8 = 2

// lookupUeByIMSI returns the attached UE context for imsi, or nil if none.
func (m *MME) lookupUeByIMSI(imsi string) *UeContext {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ue := range m.ues {
		if ue.imsi == imsi {
			return ue
		}
	}

	return nil
}

// DetachSubscriber sends a network-initiated DETACH REQUEST (TS 24.301)
// to the attached UE for imsi, if any, when a subscriber is deleted. The UE
// replies with Detach Accept, on which the S1 context is released and removed.
func (m *MME) DetachSubscriber(ctx context.Context, imsi string) {
	ue := m.lookupUeByIMSI(imsi)
	if ue == nil {
		return
	}

	logger.MmeLog.Info("network-initiated detach (subscriber deleted)",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)), zap.String("imsi", imsi))

	ue.emmState = EMMDeregistered
	m.sendDownlinkProtected(ctx, ue, &eps.DetachRequestNetwork{TypeOfDetach: detachTypeReattachNotRequired})
}

// onDetachAccept completes a network-initiated detach: the UE has acknowledged,
// so release and delete its context (the UE is already EMM-DEREGISTERED).
func (m *MME) onDetachAccept(ctx context.Context, ue *UeContext) {
	logger.MmeLog.Info("Detach Accept", zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)))
	m.releaseUEContext(ctx, ue, causeNASDetach)
}

// S1AP causes (TS 36.413) the MME uses when releasing a UE context:
// "nas: detach" after a detach, and "nas: unspecified" after an attach reject.
var (
	causeNASNormalRelease = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: 0}
	causeNASDetach        = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: 2}
	causeNASUnspecified   = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: 3}
)

// isSwitchOffDetach reports whether body is a plain UE-originating DETACH
// REQUEST with the switch-off flag set — the one NAS message the MME accepts
// without integrity protection (TS 24.301).
func isSwitchOffDetach(body []byte) bool {
	if mt, err := eps.PeekMessageType(body); err != nil || mt != eps.MsgDetachRequest {
		return false
	}

	req, err := eps.ParseDetachRequestUE(body)

	return err == nil && req.SwitchOff
}

// onDetachRequest handles a UE-originating DETACH REQUEST (TS 24.301):
// for a non-switch-off detach it replies with Detach Accept, then releases the
// UE's S1 context.
func (m *MME) onDetachRequest(ctx context.Context, ue *UeContext, plain []byte) {
	req, err := eps.ParseDetachRequestUE(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Detach Request", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Detach Request",
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
		zap.Bool("switch-off", req.SwitchOff),
		zap.String("imsi", ue.imsi),
	)

	ue.emmState = EMMDeregistered

	if !req.SwitchOff {
		m.sendDownlinkProtected(ctx, ue, &eps.DetachAccept{})
	}

	m.releaseUEContext(ctx, ue, causeNASDetach)
}

// releaseUEContext starts the S1 UE Context Release procedure for a UE
// (TS 36.413, TS 23.401). It is idempotent per UE so a detach and an
// eNB-initiated release request cannot both emit a command. Whether the context
// is deleted or retained in ECM-IDLE is decided at release-complete from the EMM
// state, since the two state machines are independent.
func (m *MME) releaseUEContext(ctx context.Context, ue *UeContext, cause s1ap.Cause) {
	// The idempotency check is atomic: a NAS guard timeout and an eNB-initiated
	// release request can race to release the same UE from different goroutines.
	m.mu.Lock()
	if ue.releasing {
		m.mu.Unlock()
		return
	}

	ue.releasing = true
	m.mu.Unlock()

	cmd := &s1ap.UEContextReleaseCommand{
		UES1APIDs: s1ap.UES1APIDs{MMEUES1APID: ue.MMEUES1APID, ENBUES1APID: ue.ENBUES1APID, Pair: true},
		Cause:     cause,
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal UE Context Release Command", zap.Error(err))
		return
	}

	logger.MmeLog.Info("UE Context Release Command", zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)))
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
		zap.Uint32("mme-ue-id", uint32(ue.MMEUES1APID)),
		zap.String("imsi", ue.imsi),
		zap.String("cause", s1apCauseName(&msg.Cause)),
	}

	// A release that arrives after the NAS security context is established but
	// before the UE is EMM-REGISTERED aborts the attach: the eNB tore down the
	// RRC connection before INITIAL CONTEXT SETUP RESPONSE and ATTACH COMPLETE,
	// so the UE will restart the whole attach. Surface it as a failure rather
	// than a routine idle release, and report whether the eNB acknowledged the
	// context setup (ics-response-received).
	if ue.secured && ue.emmState == EMMDeregistered {
		icsReceived := false
		if p := ue.defaultPDN(); p != nil {
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

	ue, ok := m.resolveUE(conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	// Independent state machines (TS 23.401): a detached UE is deleted; a
	// still-registered UE is retained in ECM-IDLE.
	if ue.emmState == EMMDeregistered {
		m.releaseAllSessions(ue)
		m.removeUe(msg.MMEUES1APID)
		logger.MmeLog.Info("UE context released", zap.Uint32("mme-ue-id", uint32(msg.MMEUES1APID)))

		return
	}

	ue.ecmState = ECMIdle
	ue.releasing = false

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
	m.mu.Lock()
	registered := ue.emmState == EMMRegistered
	imsi, mmeUEID := ue.imsi, ue.MMEUES1APID

	if registered {
		ue.ecmState = ECMIdle
		ue.releasing = false
	}

	m.mu.Unlock()

	if !registered {
		m.releaseAllSessions(ue)
		m.removeUe(mmeUEID)
		logger.MmeLog.Info("aborted incomplete UE registration",
			zap.String("trigger", trigger), zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("imsi", imsi))

		return
	}

	m.deactivateAllSessions(ue)
	m.startMobileReachable(ue)
	logger.MmeLog.Info("UE moved to ECM-IDLE",
		zap.String("trigger", trigger), zap.Uint32("mme-ue-id", uint32(mmeUEID)), zap.String("imsi", imsi))
}
