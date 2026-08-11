// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/epskeys"
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

type AdmittedERAB struct {
	Ebi      uint8
	EnbFTEID models.FTEID
}

type HandoverCandidate struct {
	Ebi   uint8
	Cause *s1ap.Cause
}

type handoverContext struct {
	state      hoState
	source     *UeConn
	target     *UeConn
	candidates []HandoverCandidate
	admitted   []AdmittedERAB
}

func (m *MME) PrepareHandover(ue *UeContext, target S1APWriter, reqMMEID s1ap.MMEUES1APID, candidates []HandoverCandidate) (targetMMEID s1ap.MMEUES1APID, newNH [32]byte, newNCC uint8, ok bool) {
	m.mu.Lock()

	if !ue.BeginKeyChainProc(procedure.S1Handover) {
		m.mu.Unlock()
		logger.MmeLog.Warn("Handover Required while a key-changing procedure is in progress",
			zap.Uint32("mme-ue-id", uint32(reqMMEID)))

		return 0, [32]byte{}, 0, false
	}

	ue.mu.Lock()

	newNH, err := epskeys.DeriveNH(ue.kasme, ue.nh[:])
	if err == nil {
		ue.nh = newNH
		ue.ncc = (ue.ncc + 1) & 0x07
		newNCC = ue.ncc
	}
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

	targetConn := &UeConn{m: m, MMEUES1APID: s1ap.MMEUES1APID(tid), ue: ue}
	targetConn.setConn(target)
	targetConn.Log = m.nodeLogLocked(target).With(logger.MMEUeS1apID(uint32(targetConn.MMEUES1APID)))
	m.conns[tid] = targetConn

	ho := &handoverContext{
		state:      hoPreparing,
		source:     ue.Conn(),
		target:     targetConn,
		candidates: candidates,
	}
	ue.handover = ho

	targetMMEID = targetConn.MMEUES1APID
	m.mu.Unlock()

	return targetMMEID, newNH, newNCC, true
}

func (m *MME) SuperviseHandover(ue *UeContext) {
	ue.SuperviseKeyChainProc(procedure.S1Handover, time.Now().Add(m.handoverGuardTimeout), func(context.Context) error {
		m.abandonHandover(ue)

		return nil
	})
}

func (m *MME) MatchAndSetTargetENB(ue *UeContext, ackMMEID s1ap.MMEUES1APID, ackENBID s1ap.ENBUES1APID, conn S1APWriter) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	if ho == nil || ho.state != hoPreparing || ho.target.MMEUES1APID != ackMMEID || ho.target.Conn() != conn {
		return false
	}

	ho.target.ENBUES1APID = ackENBID

	return true
}

func (m *MME) MarkHandoverPrepared(ue *UeContext, ackMMEID s1ap.MMEUES1APID, conn S1APWriter, admitted []AdmittedERAB) (unadmitted []HandoverCandidate, sourceConn S1APWriter, sourceMMEID s1ap.MMEUES1APID, sourceENBID s1ap.ENBUES1APID, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	if ho == nil || ho.state != hoPreparing || ho.target.MMEUES1APID != ackMMEID || ho.target.Conn() != conn {
		return nil, nil, 0, 0, false
	}

	admittedSet := make(map[uint8]struct{}, len(admitted))
	for _, a := range admitted {
		admittedSet[a.Ebi] = struct{}{}
	}

	reported := make(map[uint8]struct{}, len(ho.candidates))

	for _, c := range ho.candidates {
		if _, isAdmitted := admittedSet[c.Ebi]; isAdmitted {
			continue
		}

		if _, dup := reported[c.Ebi]; dup {
			continue
		}

		reported[c.Ebi] = struct{}{}
		unadmitted = append(unadmitted, c)
	}

	ho.admitted = admitted
	ho.state = hoPrepared

	return unadmitted, ho.source.Conn(), ho.source.MMEUES1APID, ho.source.ENBUES1APID, true
}

// HandoverTargetMatches reports whether an in-flight handover's target association
// matches the given MME-UE-S1AP-ID and connection (TS 36.413 §8.4.2.3).
func (m *MME) HandoverTargetMatches(ue *UeContext, mmeID s1ap.MMEUES1APID, conn S1APWriter) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover

	return ho != nil && ho.target.MMEUES1APID == mmeID && ho.target.Conn() == conn
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

	return ho.target.Conn(), ho.target.MMEUES1APID, ho.target.ENBUES1APID, true
}

func (m *MME) MarkHandoverCommitting(ue *UeContext, conn S1APWriter, notifyENBID s1ap.ENBUES1APID) (admitted []AdmittedERAB, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	if ho == nil || ho.state != hoPrepared || ho.target.Conn() != conn || ho.target.ENBUES1APID != notifyENBID {
		return nil, false
	}

	ho.state = hoCommitting

	return ho.admitted, true
}

func (m *MME) FinishHandoverCommit(ue *UeContext, conn S1APWriter, notifyENBID s1ap.ENBUES1APID) (sourceConn S1APWriter, sourceMMEID s1ap.MMEUES1APID, sourceENBID s1ap.ENBUES1APID, targetMMEID s1ap.MMEUES1APID, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ho := ue.handover
	if ho == nil || ho.state != hoCommitting || ho.target.Conn() != conn || ho.target.ENBUES1APID != notifyENBID || ue.Conn() == nil {
		return nil, 0, 0, 0, false
	}

	source := ho.source

	ue.active.Store(ho.target)
	source.ue = nil // its Release Complete removes the connection
	m.clearHandoverLocked(ue)

	return source.Conn(), source.MMEUES1APID, source.ENBUES1APID, ue.Conn().MMEUES1APID, true
}

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
		releaseConn, releaseMMEID, releaseENBID = ho.target.Conn(), ho.target.MMEUES1APID, ho.target.ENBUES1APID
		pair = ho.state == hoPrepared
		hasTarget = true

		m.clearHandoverLocked(ue)
	}

	return releaseConn, releaseMMEID, releaseENBID, pair, hasTarget
}

func (m *MME) BeginPathSwitch(ue *UeContext) (curNH [32]byte, curNCC uint8, mmeID s1ap.MMEUES1APID, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.Conn() != nil {
		mmeID = ue.Conn().MMEUES1APID
	}

	if !ue.BeginKeyChainProc(procedure.PathSwitch) {
		return curNH, curNCC, mmeID, false
	}

	ue.mu.Lock()
	curNH, curNCC = ue.nh, ue.ncc
	ue.mu.Unlock()

	return curNH, curNCC, mmeID, true
}

func (m *MME) ClearKeyChainBusy(ue *UeContext) {
	m.mu.Lock()
	ue.clearKeyChainProc()
	m.mu.Unlock()
}

func (m *MME) TryClaimKeyChain(ue *UeContext) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return ue.BeginKeyChainProc(procedure.SecurityMode)
}

func (m *MME) AdvancePathSwitchNH(ue *UeContext, curNH [32]byte) ([32]byte, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return epskeys.DeriveNH(ue.kasme, curNH[:])
}

func (m *MME) CommitPathSwitch(ue *UeContext, conn S1APWriter, enbUEID s1ap.ENBUES1APID, newNH [32]byte, curNCC uint8) (ncc uint8, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.Conn() == nil {
		return 0, false
	}

	ue.Conn().setConn(conn)
	ue.Conn().ENBUES1APID = enbUEID

	ue.mu.Lock()
	ue.nh = newNH
	ue.ncc = (curNCC + 1) & 0x07
	newNCC := ue.ncc
	ue.mu.Unlock()

	return newNCC, true
}

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

func (m *MME) FailHandoverToSource(ctx context.Context, ue *UeContext, cause s1ap.Cause) {
	m.mu.Lock()
	ho := ue.handover

	if ho == nil {
		m.mu.Unlock()
		return
	}

	sourceConn := ho.source.Conn()
	sourceMMEID := ho.source.MMEUES1APID
	sourceENBID := ho.source.ENBUES1APID

	m.clearHandoverLocked(ue)
	m.mu.Unlock()

	SendHandoverPreparationFailure(ctx, m, sourceConn, sourceMMEID, sourceENBID, cause)
}

func (m *MME) abandonHandover(ue *UeContext) {
	m.mu.Lock()

	ho := ue.handover
	if ho == nil || ho.state == hoCommitting {
		m.mu.Unlock()
		return
	}

	releaseTarget := ho.target
	releasePair := ho.state == hoPrepared
	sourceMMEID := ho.source.MMEUES1APID

	m.clearHandoverLocked(ue)
	m.mu.Unlock()

	logger.MmeLog.Warn("S1 handover abandoned: target did not complete it in time",
		zap.Uint32("mme-ue-id", uint32(sourceMMEID)))

	if releaseTarget != nil {
		SendUEContextRelease(context.Background(), m, releaseTarget.Conn(), releaseTarget.MMEUES1APID, releaseTarget.ENBUES1APID, releasePair, causeHandoverTS1relocExpiry)
	}
}

func (m *MME) ReleaseDetachedConn(conn S1APWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID) bool {
	m.mu.Lock()

	c, ok := m.conns[uint32(mmeUEID)]
	if !ok || c.ue != nil || c.Conn() != conn || c.ENBUES1APID != enbUEID {
		m.mu.Unlock()

		return false
	}

	m.releaseConnIDLocked(uint32(mmeUEID))

	m.mu.Unlock()

	return true
}

func SendHandoverPreparationFailure(ctx context.Context, m *MME, conn S1APWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID, cause s1ap.Cause) {
	fail := &s1ap.HandoverPreparationFailure{MMEUES1APID: s1ap.Ptr(mmeUEID), ENBUES1APID: s1ap.Ptr(enbUEID), Cause: s1ap.Ptr(cause)}

	b, err := fail.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal Handover Preparation Failure", zap.Error(err))
		return
	}

	m.SendToRadio(ctx, conn, S1APProcedureHandoverPreparationFailure, b)
}

func SendUEContextRelease(ctx context.Context, m *MME, conn S1APWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID, pair bool, cause s1ap.Cause) {
	cmd := &s1ap.UEContextReleaseCommand{
		UES1APIDs: s1ap.UES1APIDs{MMEUES1APID: mmeUEID, ENBUES1APID: enbUEID, Pair: pair},
		Cause:     new(cause),
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to marshal handover UE Context Release Command", zap.Error(err))
		return
	}

	logger.From(ctx, logger.MmeLog).Info("UE Context Release Command", zap.Uint32("mme-ue-id", uint32(mmeUEID)))
	m.SendToRadio(ctx, conn, S1APProcedureUEContextReleaseCommand, b)
}

var (
	CauseHandoverSuccess        = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkSuccessfulHandover}
	causeHandoverTS1relocExpiry = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkTS1RelocOverallExpiry}
	causeHandoverEUTRANReason   = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkReleaseDueToEUTRANGeneratedReason}
	causeHandoverCNReason       = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: s1ap.CauseRadioNetworkUnspecified}
)
