// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
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
// each with its own MME-UE-S1AP-ID; the UE's active connection (ue.s1) stays the
// source until HANDOVER NOTIFY switches it to the target. Guarded by MME.mu.
type handoverContext struct {
	state       hoState
	source      *S1Conn // the UE's source association (ue.s1 during preparation)
	target      *S1Conn // the target association; its ENBUES1APID is learned from the acknowledge
	admitted    []AdmittedERAB
	releaseEBIs []uint8 // bearers the target rejected, released at notify (TS 23.401 §5.5.1.2.2 step 15)
	// {NH, NCC} for the target, advanced at preparation, committed at notify (TS 33.401 §7.2.8).
	newNH  [32]byte
	newNCC uint8
	// guardTimer abandons the handover if the target never completes it (TS 36.413 §8.4).
	guardTimer guard.Guard
}

// PrepareHandover allocates a target association, advances the {NH, NCC} chain, and
// installs the in-flight handover on the UE (TS 36.413 §8.4.1, TS 33.401 §7.2.8).
// It refuses when the key chain is concurrently busy. reqMMEID is for logging only.
// The caller sends HANDOVER PREPARATION FAILURE on !ok.
func (m *MME) PrepareHandover(ue *UeContext, target NasWriter, reqMMEID s1ap.MMEUES1APID) (targetMMEID s1ap.MMEUES1APID, newNH [32]byte, newNCC uint8, ok bool) {
	m.mu.Lock()

	if ue.keyChainBusy {
		m.mu.Unlock()
		logger.MmeLog.Warn("Handover Required while the key chain is being advanced",
			zap.Uint32("mme-ue-id", uint32(reqMMEID)))

		return 0, [32]byte{}, 0, false
	}

	newNH, err := deriveNH(ue.kasme, ue.nh[:])
	if err != nil {
		m.mu.Unlock()
		logger.MmeLog.Error("failed to advance NH for handover", zap.Error(err))

		return 0, [32]byte{}, 0, false
	}

	newNCC = (ue.ncc + 1) & 0x07

	tid, idOK := m.allocConnIDLocked()
	if !idOK {
		m.mu.Unlock()
		return 0, [32]byte{}, 0, false
	}

	targetConn := &S1Conn{MMEUES1APID: s1ap.MMEUES1APID(tid), conn: target, ue: ue}
	m.conns[tid] = targetConn

	gen := ue.handoverGen
	ho := &handoverContext{
		state:  hoPreparing,
		source: ue.S1,
		target: targetConn,
		newNH:  newNH,
		newNCC: newNCC,
	}
	ho.guardTimer.ArmOnce(m.handoverGuardTimeout, func() { m.onHandoverGuardExpiry(ue, gen) })
	ue.handover = ho
	ue.keyChainBusy = true
	targetMMEID = targetConn.MMEUES1APID
	m.mu.Unlock()

	return targetMMEID, newNH, newNCC, true
}

// MatchAndSetTargetENB binds the target's ENB-UE-S1AP-ID to the in-flight handover
// when the acknowledge matches the preparation (TS 36.413 §8.4.2).
func (m *MME) MatchAndSetTargetENB(ue *UeContext, ackMMEID s1ap.MMEUES1APID, ackENBID s1ap.ENBUES1APID, conn NasWriter) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	if ho == nil || ho.state != hoPreparing || ho.target.MMEUES1APID != ackMMEID || ho.target.conn != conn {
		return false
	}

	ho.target.ENBUES1APID = ackENBID

	return true
}

// CommitHandoverPrepared records the admitted/rejected E-RABs and advances the
// handover to hoPrepared, returning the source association for the HANDOVER COMMAND
// (TS 36.413 §8.4.2). It re-validates the handover still matches the acknowledge.
func (m *MME) CommitHandoverPrepared(ue *UeContext, ackMMEID s1ap.MMEUES1APID, conn NasWriter, admitted []AdmittedERAB, releaseEBIs []uint8) (sourceConn NasWriter, sourceMMEID s1ap.MMEUES1APID, sourceENBID s1ap.ENBUES1APID, ok bool) {
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
func (m *MME) HandoverTargetMatches(ue *UeContext, mmeID s1ap.MMEUES1APID, conn NasWriter) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover

	return ho != nil && ho.target.MMEUES1APID == mmeID && ho.target.conn == conn
}

// HandoverStatusTarget returns the target association of an in-flight handover so
// the source's status container can be relayed (TS 36.413 §8.4.6/§8.4.7).
func (m *MME) HandoverStatusTarget(ue *UeContext) (targetConn NasWriter, targetMMEID s1ap.MMEUES1APID, targetENBID s1ap.ENBUES1APID, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	if ho == nil {
		return nil, 0, 0, false
	}

	return ho.target.conn, ho.target.MMEUES1APID, ho.target.ENBUES1APID, true
}

// BeginHandoverCommit moves a prepared handover to hoCommitting, locking out a
// concurrent CANCEL or the guard timer while the user-plane switch runs outside the
// lock, and returns the admitted/rejected E-RABs (TS 36.413 §8.4.3).
func (m *MME) BeginHandoverCommit(ue *UeContext, conn NasWriter, notifyENBID s1ap.ENBUES1APID) (admitted []AdmittedERAB, releaseEBIs []uint8, ok bool) {
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
func (m *MME) FinishHandoverCommit(ue *UeContext, conn NasWriter, notifyENBID s1ap.ENBUES1APID) (sourceConn NasWriter, sourceMMEID s1ap.MMEUES1APID, sourceENBID s1ap.ENBUES1APID, targetMMEID s1ap.MMEUES1APID, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	if ho == nil || ho.state != hoCommitting || ho.target.conn != conn || ho.target.ENBUES1APID != notifyENBID || ue.S1 == nil {
		return nil, 0, 0, 0, false
	}

	source := ho.source
	ue.nh = ho.newNH
	ue.ncc = ho.newNCC
	ue.S1 = ho.target // the target becomes the UE's active connection
	source.ue = nil   // detach the source; its Release Complete removes the connection
	m.clearHandoverLocked(ue)

	return source.conn, source.MMEUES1APID, source.ENBUES1APID, ue.S1.MMEUES1APID, true
}

// CancelHandover clears a cancellable in-flight handover, returning a prepared
// target association to release (TS 36.413 §8.4.5). A committing handover is left
// to finish.
func (m *MME) CancelHandover(ue *UeContext) (releaseConn NasWriter, releaseMMEID s1ap.MMEUES1APID, releaseENBID s1ap.ENBUES1APID, hasTarget bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	switch {
	case ho == nil:
		// Nothing to cancel; the caller still acknowledges (TS 36.413 §8.4.5.4).
	case ho.state == hoCommitting:
		// Too late to cancel: acknowledge but let the in-flight move finish.
	default:
		if ho.state == hoPrepared {
			releaseConn, releaseMMEID, releaseENBID, hasTarget = ho.target.conn, ho.target.MMEUES1APID, ho.target.ENBUES1APID, true
		}

		m.clearHandoverLocked(ue)
	}

	return releaseConn, releaseMMEID, releaseENBID, hasTarget
}

// BeginPathSwitch claims the {NH, NCC} chain for an X2 Path Switch, refusing if a
// Path Switch or S1 handover is concurrently advancing it (TS 33.401 §7.2.8). The
// claim is held until ClearKeyChainBusy. mmeID is the UE's current association id.
func (m *MME) BeginPathSwitch(ue *UeContext) (curNH [32]byte, curNCC uint8, mmeID s1ap.MMEUES1APID, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.S1 != nil {
		mmeID = ue.S1.MMEUES1APID
	}

	if ue.keyChainBusy {
		return curNH, curNCC, mmeID, false
	}

	ue.keyChainBusy = true
	curNH, curNCC = ue.nh, ue.ncc

	return curNH, curNCC, mmeID, true
}

// ClearKeyChainBusy releases the {NH, NCC} chain claim taken by BeginPathSwitch
// or the NAS security mode procedure.
func (m *MME) ClearKeyChainBusy(ue *UeContext) {
	m.mu.Lock()
	ue.keyChainBusy = false
	m.mu.Unlock()
}

// TryClaimKeyChain claims the {NH, NCC} key chain for ue so a key-changing
// procedure — a NAS security mode, Path Switch, or S1 handover — cannot run
// concurrently with another and desync the AS/NAS key chain (TS 33.501 §6.9.5.1,
// TS 33.401 §7.2.8). It returns false when the chain is already claimed. The
// claim is released by ClearKeyChainBusy, by handover/path-switch completion, or
// when the connection is freed.
func (m *MME) TryClaimKeyChain(ue *UeContext) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.keyChainBusy {
		return false
	}

	ue.keyChainBusy = true

	return true
}

// AdvancePathSwitchNH derives the next hop for a Path Switch from the current NH
// (TS 33.401 §7.2.8). kasme stays inside the kernel.
func (m *MME) AdvancePathSwitchNH(ue *UeContext, curNH [32]byte) ([32]byte, error) {
	return deriveNH(ue.kasme, curNH[:])
}

// CommitPathSwitch moves the UE's active association to the target eNB and commits
// the advanced {NH, NCC} chain (TS 36.413, TS 33.401 §7.2.8). ok is false if the UE
// was released during the unlocked user-plane switch.
func (m *MME) CommitPathSwitch(ue *UeContext, conn NasWriter, enbUEID s1ap.ENBUES1APID, newNH [32]byte, curNCC uint8) (ncc uint8, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.S1 == nil {
		return 0, false
	}

	ue.S1.conn = conn
	ue.S1.ENBUES1APID = enbUEID
	ue.nh = newNH
	ue.ncc = (curNCC + 1) & 0x07

	return ue.ncc, true
}

// clearHandoverLocked drops the UE's in-flight handover context, stops its guard
// timer, and removes the target connection it allocated — unless the handover
// completed and the UE moved onto the target (ue.s1). It bumps handoverGen so a
// guard callback that fired concurrently is recognised as stale. The caller holds
// MME.mu.
func (m *MME) clearHandoverLocked(ue *UeContext) {
	ho := ue.handover
	if ho == nil {
		return
	}

	ho.guardTimer.Stop()

	if ho.target != nil && ho.target != ue.S1 {
		ho.target.ue = nil
		m.releaseConnIDLocked(uint32(ho.target.MMEUES1APID))
	}

	ue.handover = nil
	ue.handoverGen++
	ue.keyChainBusy = false
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

// onHandoverGuardExpiry abandons a handover whose target never completed it
// (TS 36.413 §8.4): the UE stays on the source eNB and a prepared target's resources
// are released. gen guards against a callback that fired just as the handover was
// cleared or replaced. A handover already committing (the UE has reached the
// target) is left to finish.
func (m *MME) onHandoverGuardExpiry(ue *UeContext, gen uint64) {
	m.mu.Lock()

	ho := ue.handover
	if ho == nil || ue.handoverGen != gen || ho.state == hoCommitting {
		m.mu.Unlock()
		return
	}

	var releaseTarget *S1Conn
	if ho.state == hoPrepared {
		releaseTarget = ho.target
	}

	sourceMMEID := ho.source.MMEUES1APID

	m.clearHandoverLocked(ue)
	m.mu.Unlock()

	logger.MmeLog.Warn("S1 handover abandoned: target did not complete it in time",
		zap.Uint32("mme-ue-id", uint32(sourceMMEID)))

	if releaseTarget != nil {
		SendUEContextRelease(m, context.Background(), releaseTarget.conn, releaseTarget.MMEUES1APID, releaseTarget.ENBUES1APID)
	}
}

// ReleaseDetachedConn removes a UE-associated connection that holds no UE context —
// a handover source detached at HANDOVER NOTIFY, or a released target — when its UE
// Context Release Complete arrives, identified by its own MME-UE-S1AP-ID (TS 36.413
// §8.4). It reports whether it handled one.
func (m *MME) ReleaseDetachedConn(conn NasWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.conns[uint32(mmeUEID)]
	if !ok || c.ue != nil || c.conn != conn || c.ENBUES1APID != enbUEID {
		return false
	}

	m.releaseConnIDLocked(uint32(mmeUEID))

	return true
}

func SendHandoverPreparationFailure(m *MME, ctx context.Context, conn NasWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID, cause s1ap.Cause) {
	fail := &s1ap.HandoverPreparationFailure{MMEUES1APID: mmeUEID, ENBUES1APID: enbUEID, Cause: cause}

	b, err := fail.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Handover Preparation Failure", zap.Error(err))
		return
	}

	m.SendS1APConn(ctx, conn, S1APProcedureHandoverPreparationFailure, b)
}

func SendUEContextRelease(m *MME, ctx context.Context, conn NasWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID) {
	cmd := &s1ap.UEContextReleaseCommand{
		UES1APIDs: s1ap.UES1APIDs{MMEUES1APID: mmeUEID, ENBUES1APID: enbUEID, Pair: true},
		Cause:     causeSuccessfulHandover,
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal handover UE Context Release Command", zap.Error(err))
		return
	}

	logger.MmeLog.Info("UE Context Release Command (handover)", zap.Uint32("mme-ue-id", uint32(mmeUEID)))
	m.SendS1APConn(ctx, conn, S1APProcedureUEContextReleaseCommand, b)
}

// causeSuccessfulHandover is S1AP Cause Radio Network "successful-handover"
// (TS 36.413), used in the UE Context Release Command for the source eNB.
var causeSuccessfulHandover = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 2}
