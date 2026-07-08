// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme/procedure"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// hoState tracks where an S1 handover is in its preparation (TS 36.413 §8.4).
type hoState uint8

const (
	hoPreparing  hoState = iota // HANDOVER REQUEST sent, awaiting acknowledge
	hoPrepared                  // HANDOVER COMMAND sent, awaiting notify
	hoCommitting                // HANDOVER NOTIFY received, the user-plane switch is in progress
)

// AdmittedERAB is one E-RAB the target eNB admitted: its EPS bearer identity and
// the target's S1-U downlink endpoint.
type AdmittedERAB struct {
	Ebi      uint8
	EnbFTEID models.FTEID
}

// handoverContext is the MME's state for one in-flight inter-eNB S1 handover
// (TS 36.413 §8.4). source and target are distinct UE-associated S1-connections,
// each with its own MME-UE-S1AP-ID; the UE's active connection (ue.active) stays the
// source until HANDOVER NOTIFY switches it to the target. Guarded by MME.mu.
type handoverContext struct {
	state       hoState
	source      *UeConn // the UE's source association (ue.active during preparation)
	target      *UeConn // its ENBUES1APID is learned from the acknowledge
	admitted    []AdmittedERAB
	releaseEBIs []uint8 // bearers the target rejected, released at notify (TS 23.401 §5.5.1.2.2 step 15)
	// {NH, NCC} for the target, advanced at preparation, committed at notify (TS 33.401 §7.2.8).
	newNH  [32]byte
	newNCC uint8
}

// PrepareHandover allocates a target association, advances the {NH, NCC} chain, and
// installs the in-flight handover on the UE (TS 36.413 §8.4.1, TS 33.401 §7.2.8).
// It refuses when the key chain is concurrently busy. reqMMEID is for logging only.
func (m *MME) PrepareHandover(ue *UeContext, target S1APWriter, reqMMEID s1ap.MMEUES1APID) (targetMMEID s1ap.MMEUES1APID, newNH [32]byte, newNCC uint8, ok bool) {
	m.mu.Lock()

	if !ue.BeginKeyChainProc(context.Background(), procedure.S1Handover) {
		m.mu.Unlock()
		logger.MmeLog.Warn("Handover Required while a key-changing procedure is in progress",
			zap.Uint32("mme-ue-id", uint32(reqMMEID)))

		return 0, [32]byte{}, 0, false
	}

	ue.mu.Lock()
	newNH, err := deriveNH(ue.kasme, ue.nh[:])
	newNCC = (ue.ncc + 1) & 0x07
	ue.mu.Unlock()

	if err != nil {
		ue.EndKeyChainProc(procedure.S1Handover)
		m.mu.Unlock()
		logger.MmeLog.Error("failed to advance NH for handover", zap.Error(err))

		return 0, [32]byte{}, 0, false
	}

	tid, idOK := m.allocConnIDLocked()
	if !idOK {
		ue.EndKeyChainProc(procedure.S1Handover)
		m.mu.Unlock()

		return 0, [32]byte{}, 0, false
	}

	targetConn := &UeConn{m: m, MMEUES1APID: s1ap.MMEUES1APID(tid), conn: target, ue: ue}
	targetConn.Log = m.nodeLogLocked(target).With(logger.MMEUeS1apID(uint32(targetConn.MMEUES1APID)))
	m.conns[tid] = targetConn

	ho := &handoverContext{
		state:  hoPreparing,
		source: ue.Conn(),
		target: targetConn,
		newNH:  newNH,
		newNCC: newNCC,
	}
	ue.handover = ho
	// Supervise the S1Handover procedure: at TS1RELOCoverall expiry the registry runs
	// abandonHandover while S1Handover is still active (TS 36.413 §8.4).
	ue.SuperviseKeyChainProc(context.Background(), procedure.S1Handover, time.Now().Add(m.handoverGuardTimeout), func(context.Context) error {
		m.abandonHandover(ue)

		return nil
	})

	targetMMEID = targetConn.MMEUES1APID
	m.mu.Unlock()

	return targetMMEID, newNH, newNCC, true
}

// MatchAndSetTargetENB binds the target's ENB-UE-S1AP-ID to the in-flight handover
// when the acknowledge matches the preparation (TS 36.413 §8.4.2).
func (m *MME) MatchAndSetTargetENB(ue *UeContext, ackMMEID s1ap.MMEUES1APID, ackENBID s1ap.ENBUES1APID, conn S1APWriter) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	if ho == nil || ho.state != hoPreparing || ho.target.MMEUES1APID != ackMMEID || ho.target.conn != conn {
		return false
	}

	ho.target.ENBUES1APID = ackENBID

	return true
}

// MarkHandoverPrepared records the admitted/rejected E-RABs and advances the
// handover to hoPrepared, returning the source association for the HANDOVER COMMAND
// (TS 36.413 §8.4.2). It re-validates the handover still matches the acknowledge.
func (m *MME) MarkHandoverPrepared(ue *UeContext, ackMMEID s1ap.MMEUES1APID, conn S1APWriter, admitted []AdmittedERAB, releaseEBIs []uint8) (sourceConn S1APWriter, sourceMMEID s1ap.MMEUES1APID, sourceENBID s1ap.ENBUES1APID, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	if ho == nil || ho.state != hoPreparing || ho.target.MMEUES1APID != ackMMEID || ho.target.conn != conn {
		return nil, 0, 0, false
	}

	ho.admitted = admitted
	ho.releaseEBIs = releaseEBIs
	ho.state = hoPrepared

	return ho.source.conn, ho.source.MMEUES1APID, ho.source.ENBUES1APID, true
}

// HandoverTargetMatches reports whether an in-flight handover's target association
// matches the given MME-UE-S1AP-ID and connection (TS 36.413 §8.4.2.3).
func (m *MME) HandoverTargetMatches(ue *UeContext, mmeID s1ap.MMEUES1APID, conn S1APWriter) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover

	return ho != nil && ho.target.MMEUES1APID == mmeID && ho.target.conn == conn
}

// HandoverStatusTarget returns the target association of an in-flight handover so
// the source's status container can be relayed (TS 36.413 §8.4.6/§8.4.7).
func (m *MME) HandoverStatusTarget(ue *UeContext) (targetConn S1APWriter, targetMMEID s1ap.MMEUES1APID, targetENBID s1ap.ENBUES1APID, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	if ho == nil {
		return nil, 0, 0, false
	}

	return ho.target.conn, ho.target.MMEUES1APID, ho.target.ENBUES1APID, true
}

// MarkHandoverCommitting moves a prepared handover to hoCommitting, locking out a
// concurrent CANCEL or the supervision timeout while the user-plane switch runs outside
// the lock, and returns the admitted/rejected E-RABs (TS 36.413 §8.4.3).
func (m *MME) MarkHandoverCommitting(ue *UeContext, conn S1APWriter, notifyENBID s1ap.ENBUES1APID) (admitted []AdmittedERAB, releaseEBIs []uint8, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	if ho == nil || ho.state != hoPrepared || ho.target.conn != conn || ho.target.ENBUES1APID != notifyENBID {
		return nil, nil, false
	}

	ho.state = hoCommitting

	return ho.admitted, ho.releaseEBIs, true
}

// FinishHandoverCommit commits the {NH, NCC} chain, switches the UE's active S1
// connection to the target, detaches the source, and clears the handover
// (TS 36.413 §8.4.3, TS 33.401 §7.2.8). It returns the source association to
// release. ok is false if a concurrent release tore the UE down during the switch.
func (m *MME) FinishHandoverCommit(ue *UeContext, conn S1APWriter, notifyENBID s1ap.ENBUES1APID) (sourceConn S1APWriter, sourceMMEID s1ap.MMEUES1APID, sourceENBID s1ap.ENBUES1APID, targetMMEID s1ap.MMEUES1APID, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	if ho == nil || ho.state != hoCommitting || ho.target.conn != conn || ho.target.ENBUES1APID != notifyENBID || ue.Conn() == nil {
		return nil, 0, 0, 0, false
	}

	source := ho.source

	ue.mu.Lock()
	ue.nh = ho.newNH
	ue.ncc = ho.newNCC
	ue.mu.Unlock()

	ue.active.Store(ho.target)
	source.ue = nil // its Release Complete removes the connection
	m.clearHandoverLocked(ue)

	return source.conn, source.MMEUES1APID, source.ENBUES1APID, ue.Conn().MMEUES1APID, true
}

// CancelHandover clears a cancellable in-flight handover, returning the reserved
// target association to release (TS 36.413 §8.4.5). The target is released whether it
// has acknowledged (hoPrepared, pair true — its eNB-UE-S1AP-ID is known) or is still
// preparing (hoPreparing, pair false — addressed by MME-UE-S1AP-ID alone), so a cancel
// never orphans the target's reserved resources. A committing handover is left to finish.
func (m *MME) CancelHandover(ue *UeContext) (releaseConn S1APWriter, releaseMMEID s1ap.MMEUES1APID, releaseENBID s1ap.ENBUES1APID, pair, hasTarget bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	switch {
	case ho == nil:
		// Nothing to cancel; the caller still acknowledges (TS 36.413 §8.4.5.4).
	case ho.state == hoCommitting:
		// Too late to cancel: acknowledge but let the in-flight move finish.
	default:
		releaseConn, releaseMMEID, releaseENBID = ho.target.conn, ho.target.MMEUES1APID, ho.target.ENBUES1APID
		pair = ho.state == hoPrepared
		hasTarget = true

		m.clearHandoverLocked(ue)
	}

	return releaseConn, releaseMMEID, releaseENBID, pair, hasTarget
}

// BeginPathSwitch claims the {NH, NCC} chain for an X2 Path Switch, refusing if a
// Path Switch or S1 handover is concurrently advancing it (TS 33.401 §7.2.8). The
// claim is held until ClearKeyChainBusy. mmeID is the UE's current association id.
func (m *MME) BeginPathSwitch(ue *UeContext) (curNH [32]byte, curNCC uint8, mmeID s1ap.MMEUES1APID, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.Conn() != nil {
		mmeID = ue.Conn().MMEUES1APID
	}

	if !ue.BeginKeyChainProc(context.Background(), procedure.PathSwitch) {
		return curNH, curNCC, mmeID, false
	}

	ue.mu.Lock()
	curNH, curNCC = ue.nh, ue.ncc
	ue.mu.Unlock()

	return curNH, curNCC, mmeID, true
}

// ClearKeyChainBusy releases the {NH, NCC} chain claim taken by BeginPathSwitch
// or the NAS security mode procedure. The active key-changing procedure is ended;
// they are mutually exclusive, so at most one is cleared.
func (m *MME) ClearKeyChainBusy(ue *UeContext) {
	m.mu.Lock()
	ue.clearKeyChainProc()
	m.mu.Unlock()
}

// TryClaimKeyChain claims the {NH, NCC} key chain for the NAS security mode
// procedure so it cannot run concurrently with a Path Switch or S1 handover and
// desync the AS/NAS key chain (TS 33.501 §6.9.5.1, TS 33.401 §7.2.8). It returns
// false when the chain is already claimed. The claim is released by
// ClearKeyChainBusy, by handover/path-switch completion, or when the connection is
// freed.
func (m *MME) TryClaimKeyChain(ue *UeContext) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return ue.BeginKeyChainProc(context.Background(), procedure.SecurityMode)
}

// AdvancePathSwitchNH derives the next hop for a Path Switch from the current NH
// (TS 33.401 §7.2.8). kasme stays inside the kernel.
func (m *MME) AdvancePathSwitchNH(ue *UeContext, curNH [32]byte) ([32]byte, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return deriveNH(ue.kasme, curNH[:])
}

// CommitPathSwitch moves the UE's active association to the target eNB and commits
// the advanced {NH, NCC} chain (TS 36.413, TS 33.401 §7.2.8). ok is false if the UE
// was released during the unlocked user-plane switch.
func (m *MME) CommitPathSwitch(ue *UeContext, conn S1APWriter, enbUEID s1ap.ENBUES1APID, newNH [32]byte, curNCC uint8) (ncc uint8, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.Conn() == nil {
		return 0, false
	}

	ue.Conn().conn = conn
	ue.Conn().ENBUES1APID = enbUEID

	ue.mu.Lock()
	ue.nh = newNH
	ue.ncc = (curNCC + 1) & 0x07
	newNCC := ue.ncc
	ue.mu.Unlock()

	return newNCC, true
}

// clearHandoverLocked drops the UE's in-flight handover context and removes the
// target connection it allocated — unless the handover completed and the UE moved
// onto the target (ue.active). Ending the S1Handover procedure stops its registry
// supervision timer. The caller holds MME.mu.
func (m *MME) clearHandoverLocked(ue *UeContext) {
	ho := ue.handover
	if ho == nil {
		return
	}

	if ho.target != nil && ho.target != ue.Conn() {
		ho.target.ue = nil
		m.releaseConnIDLocked(uint32(ho.target.MMEUES1APID))
	}

	ue.handover = nil
	ue.EndKeyChainProc(procedure.S1Handover)
}

// ClearHandover drops the UE's in-flight handover context under MME.mu.
func (m *MME) ClearHandover(ue *UeContext) {
	m.mu.Lock()
	m.clearHandoverLocked(ue)
	m.mu.Unlock()
}

// FailHandoverToSource clears the handover and sends a HANDOVER PREPARATION
// FAILURE to the source eNB, leaving the UE on the source association. The target
// connection allocated for the preparation is dropped by clearHandoverLocked.
func (m *MME) FailHandoverToSource(ctx context.Context, ue *UeContext, cause s1ap.Cause) {
	m.mu.Lock()
	ho := ue.handover

	if ho == nil {
		m.mu.Unlock()
		return
	}

	sourceConn := ho.source.conn
	sourceMMEID := ho.source.MMEUES1APID
	sourceENBID := ho.source.ENBUES1APID

	m.clearHandoverLocked(ue)
	m.mu.Unlock()

	SendHandoverPreparationFailure(m, ctx, sourceConn, sourceMMEID, sourceENBID, cause)
}

// abandonHandover is the S1Handover supervision-timeout action (TS 36.413 §8.4): the
// target never completed the handover, so the UE stays on the source eNB and a prepared
// target's resources are released. The registry invokes this only while S1Handover is
// still active (it re-checks the procedure id under its own lock), so no generation
// counter is needed here. A handover already committing (the UE has reached the target)
// is left to finish.
func (m *MME) abandonHandover(ue *UeContext) {
	m.mu.Lock()

	ho := ue.handover
	if ho == nil || ho.state == hoCommitting {
		m.mu.Unlock()
		return
	}

	// Release the reserved target whether it acknowledged (hoPrepared) or was still
	// preparing when the guard fired: either way it holds resources reserved by the
	// HANDOVER REQUEST that must be freed (TS 36.413 §8.4). A preparing target is
	// addressed by its MME-UE-S1AP-ID alone (its eNB-UE-S1AP-ID never arrived).
	releaseTarget := ho.target
	releasePair := ho.state == hoPrepared
	sourceMMEID := ho.source.MMEUES1APID

	m.clearHandoverLocked(ue)
	m.mu.Unlock()

	logger.MmeLog.Warn("S1 handover abandoned: target did not complete it in time",
		zap.Uint32("mme-ue-id", uint32(sourceMMEID)))

	if releaseTarget != nil {
		SendUEContextRelease(m, context.Background(), releaseTarget.conn, releaseTarget.MMEUES1APID, releaseTarget.ENBUES1APID, releasePair, causeHandoverTS1relocExpiry)
	}
}

// ReleaseDetachedConn removes a UE-associated connection that holds no UE context —
// a handover source detached at HANDOVER NOTIFY, or a released target — when its UE
// Context Release Complete arrives, identified by its own MME-UE-S1AP-ID (TS 36.413
// §8.4). It reports whether it handled one.
func (m *MME) ReleaseDetachedConn(conn S1APWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.conns[uint32(mmeUEID)]
	if !ok || c.ue != nil || c.conn != conn || c.ENBUES1APID != enbUEID {
		return false
	}

	m.releaseConnIDLocked(uint32(mmeUEID))

	return true
}

func SendHandoverPreparationFailure(m *MME, ctx context.Context, conn S1APWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID, cause s1ap.Cause) {
	fail := &s1ap.HandoverPreparationFailure{MMEUES1APID: mmeUEID, ENBUES1APID: enbUEID, Cause: cause}

	b, err := fail.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal Handover Preparation Failure", zap.Error(err))
		return
	}

	m.SendS1APConn(ctx, conn, S1APProcedureHandoverPreparationFailure, b)
}

// SendUEContextRelease sends a UE Context Release Command over conn. pair selects the
// UE S1AP IDs alternative: the full pair when the eNB-UE-S1AP-ID is known, or the
// MME-UE-S1AP-ID alone for a still-preparing target whose eNB-UE-S1AP-ID has not yet
// arrived in a HANDOVER REQUEST ACKNOWLEDGE (TS 36.413 §9.1.4.5).
func SendUEContextRelease(m *MME, ctx context.Context, conn S1APWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID, pair bool, cause s1ap.Cause) {
	cmd := &s1ap.UEContextReleaseCommand{
		UES1APIDs: s1ap.UES1APIDs{MMEUES1APID: mmeUEID, ENBUES1APID: enbUEID, Pair: pair},
		Cause:     cause,
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal handover UE Context Release Command", zap.Error(err))
		return
	}

	logger.From(ctx, logger.MmeLog).Info("UE Context Release Command (handover)", zap.Uint32("mme-ue-id", uint32(mmeUEID)))
	m.SendS1APConn(ctx, conn, S1APProcedureUEContextReleaseCommand, b)
}

// S1AP Cause Radio Network values used when releasing an eNB's UE context during a
// handover (TS 36.413 §9.2.1.3, enumeration order). Only the source release after
// a completed handover is "successful-handover"; abandoning a prepared handover
// uses a cause that reflects why.
var (
	// CauseHandoverSuccess releases the source eNB after HANDOVER NOTIFY.
	CauseHandoverSuccess = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 2} // successful-handover
	// causeHandoverTS1relocExpiry releases a prepared target the supervision timeout abandons.
	causeHandoverTS1relocExpiry = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 8} // tS1relocoverall-expiry
	// causeHandoverEUTRANReason releases a prepared target when the source association drops.
	causeHandoverEUTRANReason = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 3} // release-due-to-eutran-generated-reason
)
