// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// S1AP causes the MME returns during S1 handover (TS 36.413 §9.2.1.3,
// CauseRadioNetwork enumeration).
var (
	causeSuccessfulHandover     = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 2}  // successful-handover
	causeHOFailureInTarget      = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 6}  // ho-failure-in-target-EPC-eNB-or-target-system
	causeHOTargetNotAllowed     = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 7}  // ho-target-not-allowed
	causeUnknownTargetID        = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 11} // unknown-targetID
	causeHandoverNoSecurity     = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: 1}           // authentication-failure
	causeHandoverPrepUnspecific = s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}  // unspecified
)

// hoState tracks where an S1 handover is in its preparation (TS 36.413 §8.4).
type hoState uint8

const (
	hoPreparing  hoState = iota // HANDOVER REQUEST sent, awaiting acknowledge
	hoPrepared                  // HANDOVER COMMAND sent, awaiting notify
	hoCommitting                // HANDOVER NOTIFY received, the user-plane switch is in progress
)

// admittedERAB is one E-RAB the target eNB admitted: its EPS bearer identity and
// the target's S1-U downlink endpoint.
type admittedERAB struct {
	ebi      uint8
	enbFTEID models.FTEID
}

// handoverContext is the MME's state for one in-flight inter-eNB S1 handover
// (TS 36.413 §8.4). source and target are distinct UE-associated S1-connections,
// each with its own MME-UE-S1AP-ID; the UE's active connection (ue.s1) stays the
// source until HANDOVER NOTIFY switches it to the target. Guarded by MME.mu.
type handoverContext struct {
	state       hoState
	source      *s1Conn // the UE's source association (ue.s1 during preparation)
	target      *s1Conn // the target association; its ENBUES1APID is learned from the acknowledge
	admitted    []admittedERAB
	releaseEBIs []uint8 // bearers the target rejected, released at notify (TS 23.401 §5.5.1.2.2 step 15)
	// {NH, NCC} for the target, advanced at preparation, committed at notify (TS 33.401 §7.2.8).
	newNH  [32]byte
	newNCC uint8
	// guardTimer abandons the handover if the target never completes it (TS 36.413 §8.4).
	guardTimer *time.Timer
}

// handleHandoverRequired starts an S1 handover preparation toward the target eNB,
// or replies HANDOVER PREPARATION FAILURE (TS 36.413 §8.4.1). conn is the source.
func (m *MME) handleHandoverRequired(ctx context.Context, conn nasWriter, value []byte) {
	req, err := s1ap.ParseHandoverRequired(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Required", zap.Error(err))
		return
	}

	ue, ok := m.resolveUE(conn, req.MMEUES1APID, req.ENBUES1APID)
	if !ok {
		return
	}

	if req.HandoverType != s1ap.HandoverTypeIntraLTE {
		logger.MmeLog.Warn("Handover Required for an unsupported handover type",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)), zap.Uint8("handover-type", uint8(req.HandoverType)))
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHOTargetNotAllowed)

		return
	}

	if !ue.secured || len(ue.kasme) == 0 {
		logger.MmeLog.Warn("Handover Required for a UE without a security context",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)))
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverNoSecurity)

		return
	}

	target, ok := m.findENBByGlobalID(req.TargetID.TargeteNBID.GlobalENBID)
	if !ok {
		logger.MmeLog.Warn("Handover Required for an unknown target eNB",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)), zap.String("target-enb", enbID(req.TargetID.TargeteNBID.GlobalENBID)))
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeUnknownTargetID)

		return
	}

	if target == conn {
		logger.MmeLog.Warn("Handover Required targets the source eNB",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)))
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHOTargetNotAllowed)

		return
	}

	bearers, ok := m.handoverBearers(ue)
	if !ok {
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)
		return
	}

	// One handover at a time per UE, and the key chain must not be advanced
	// concurrently by a Path Switch (TS 33.401 §7.2.8). The target connection gets a
	// fresh MME-UE-S1AP-ID so it is a distinct UE-associated logical connection from
	// the source (TS 36.413); the {NH, NCC} is advanced here and committed at notify.
	m.mu.Lock()
	if ue.keyChainBusy {
		m.mu.Unlock()
		logger.MmeLog.Warn("Handover Required while the key chain is being advanced",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)))
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)

		return
	}

	newNH, err := deriveNH(ue.kasme, ue.nh[:])
	if err != nil {
		m.mu.Unlock()
		logger.MmeLog.Error("failed to advance NH for handover", zap.Error(err))
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)

		return
	}

	newNCC := (ue.ncc + 1) & 0x07

	tid := m.nextMMEUEID
	m.nextMMEUEID++
	targetConn := &s1Conn{MMEUES1APID: s1ap.MMEUES1APID(tid), conn: target, ue: ue}
	m.conns[tid] = targetConn

	gen := ue.handoverGen
	ho := &handoverContext{
		state:  hoPreparing,
		source: ue.s1,
		target: targetConn,
		newNH:  newNH,
		newNCC: newNCC,
	}
	ho.guardTimer = time.AfterFunc(m.handoverGuardTimeout, func() { m.onHandoverGuardExpiry(ue, gen) })
	ue.handover = ho
	ue.keyChainBusy = true
	targetMMEID := targetConn.MMEUES1APID
	m.mu.Unlock()

	hoReq := &s1ap.HandoverRequest{
		MMEUES1APID:            targetMMEID,
		HandoverType:           s1ap.HandoverTypeIntraLTE,
		Cause:                  req.Cause,
		UEAMBR:                 m.handoverUEAMBR(ue),
		ERABToBeSetup:          bearers,
		SourceToTarget:         req.SourceToTarget,
		UESecurityCapabilities: m.handoverSecurityCapabilities(ue),
		SecurityContext:        s1ap.SecurityContext{NextHopChainingCount: newNCC, NextHopParameter: s1ap.SecurityKey(newNH)},
	}

	b, err := hoReq.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Handover Request", zap.Error(err))
		m.clearHandover(ue)
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)

		return
	}

	logger.MmeLog.Info("Handover Request",
		zap.Uint32("target-mme-ue-id", uint32(targetMMEID)),
		zap.String("target-enb", enbID(req.TargetID.TargeteNBID.GlobalENBID)),
		zap.Int("e-rabs", len(bearers)))
	m.sendS1APConn(ctx, target, S1APProcedureHandoverRequest, b)
}

// handleHandoverRequestAcknowledge records the target's downlink endpoints and
// sends a HANDOVER COMMAND to the source, or fails the handover when no usable
// bearer was admitted (TS 36.413 §8.4.2). conn is the target; the acknowledge
// carries the target's MME-UE-S1AP-ID.
func (m *MME) handleHandoverRequestAcknowledge(ctx context.Context, conn nasWriter, value []byte) {
	ack, err := s1ap.ParseHandoverRequestAcknowledge(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Request Acknowledge", zap.Error(err))
		return
	}

	ue, ok := m.lookupUe(ack.MMEUES1APID)
	if !ok {
		// No UE for this target id; release the context the ack just created.
		m.sendUEContextRelease(ctx, conn, ack.MMEUES1APID, ack.ENBUES1APID)
		return
	}

	m.mu.Lock()
	ho := ue.handover

	if ho == nil || ho.state != hoPreparing || ho.target.MMEUES1APID != ack.MMEUES1APID || ho.target.conn != conn {
		m.mu.Unlock()
		logger.MmeLog.Warn("Handover Request Acknowledge with no matching preparation",
			zap.Uint32("target-mme-ue-id", uint32(ack.MMEUES1APID)))
		m.sendUEContextRelease(ctx, conn, ack.MMEUES1APID, ack.ENBUES1APID)

		return
	}

	ho.target.ENBUES1APID = ack.ENBUES1APID
	m.mu.Unlock()

	admitted := make([]admittedERAB, 0, len(ack.ERABAdmitted))

	for _, it := range ack.ERABAdmitted {
		addr, ok := enbTransportAddress(it.TransportLayerAddress)
		if !ok {
			logger.MmeLog.Warn("Handover Request Acknowledge E-RAB has an invalid target address; treating as failed",
				zap.Uint32("target-mme-ue-id", uint32(ack.MMEUES1APID)), zap.Uint8("e-rab-id", uint8(it.ERABID)))

			continue
		}

		admitted = append(admitted, admittedERAB{ebi: uint8(it.ERABID), enbFTEID: models.FTEID{TEID: uint32(it.GTPTEID), Addr: addr}})
	}

	// Each E-RAB is a PDN's default bearer, so a rejected one releases its PDN
	// (TS 23.401 §5.5.1.2.2 step 15).
	releaseEBIs := failedHandoverEBIs(ack, admitted)

	if len(admitted) == 0 {
		// No default bearer admitted: the handover is rejected (TS 23.401 §5.5.1.2.3).
		logger.MmeLog.Warn("Handover Request Acknowledge admitted no E-RAB; rejecting handover",
			zap.Uint32("target-mme-ue-id", uint32(ack.MMEUES1APID)))
		m.sendUEContextRelease(ctx, conn, ack.MMEUES1APID, ack.ENBUES1APID)
		m.failHandoverToSource(ctx, ue, causeHOFailureInTarget)

		return
	}

	m.mu.Lock()
	if ue.handover != ho || ho.state != hoPreparing {
		m.mu.Unlock()
		return
	}

	ho.admitted = admitted
	ho.releaseEBIs = releaseEBIs
	ho.state = hoPrepared
	sourceConn := ho.source.conn
	sourceMMEID := ho.source.MMEUES1APID
	sourceENBID := ho.source.ENBUES1APID
	m.mu.Unlock()

	cmd := &s1ap.HandoverCommand{
		MMEUES1APID:    sourceMMEID,
		ENBUES1APID:    sourceENBID,
		HandoverType:   s1ap.HandoverTypeIntraLTE,
		ERABToRelease:  releaseItems(releaseEBIs),
		TargetToSource: ack.TargetToSource,
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Handover Command", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Handover Command",
		zap.Uint32("mme-ue-id", uint32(sourceMMEID)),
		zap.Int("admitted", len(admitted)),
		zap.Int("released", len(releaseEBIs)))
	m.sendS1APConn(ctx, sourceConn, S1APProcedureHandoverCommand, b)
}

// handleHandoverFailure fails the preparation toward the source when the target
// could not admit the handover, leaving the UE on the source (TS 36.413 §8.4.2.3).
// conn is the target; the failure carries the target's MME-UE-S1AP-ID.
func (m *MME) handleHandoverFailure(ctx context.Context, conn nasWriter, value []byte) {
	fail, err := s1ap.ParseHandoverFailure(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Failure", zap.Error(err))
		return
	}

	ue, ok := m.lookupUe(fail.MMEUES1APID)
	if !ok {
		return
	}

	m.mu.Lock()
	ho := ue.handover

	if ho == nil || ho.target.MMEUES1APID != fail.MMEUES1APID || ho.target.conn != conn {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	logger.MmeLog.Info("Handover Failure", zap.Uint32("target-mme-ue-id", uint32(fail.MMEUES1APID)))
	m.failHandoverToSource(ctx, ue, causeHOFailureInTarget)
}

// handleENBStatusTransfer relays the source's status container to the target as an
// MME STATUS TRANSFER (TS 36.413 §8.4.6/§8.4.7). Optional: the source may omit it,
// so it never gates completion. conn is the source.
func (m *MME) handleENBStatusTransfer(ctx context.Context, conn nasWriter, value []byte) {
	st, err := s1ap.ParseENBStatusTransfer(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode eNB Status Transfer", zap.Error(err))
		return
	}

	ue, ok := m.resolveUE(conn, st.MMEUES1APID, st.ENBUES1APID)
	if !ok {
		return
	}

	m.mu.Lock()
	ho := ue.handover

	if ho == nil {
		m.mu.Unlock()
		logger.MmeLog.Warn("eNB Status Transfer with no handover in progress", zap.Uint32("mme-ue-id", uint32(st.MMEUES1APID)))

		return
	}

	targetConn := ho.target.conn
	targetMMEID := ho.target.MMEUES1APID
	targetENBID := ho.target.ENBUES1APID
	m.mu.Unlock()

	mst := &s1ap.MMEStatusTransfer{MMEUES1APID: targetMMEID, ENBUES1APID: targetENBID, Container: st.Container}

	b, err := mst.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal MME Status Transfer", zap.Error(err))
		return
	}

	m.sendS1APConn(ctx, targetConn, S1APProcedureMMEStatusTransfer, b)
}

// handleHandoverNotify completes the handover once the UE reaches the target: it
// switches the user plane, commits the {NH, NCC} chain, moves the active S1
// connection to the target, and releases the source by its own MME-UE-S1AP-ID
// (TS 36.413 §8.4.3, TS 23.401 §5.5.1.2.2 steps 13-19). conn is the target.
func (m *MME) handleHandoverNotify(ctx context.Context, conn nasWriter, value []byte) {
	notify, err := s1ap.ParseHandoverNotify(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Notify", zap.Error(err))
		return
	}

	ue, ok := m.lookupUe(notify.MMEUES1APID)
	if !ok {
		return
	}

	m.mu.Lock()
	ho := ue.handover

	if ho == nil || ho.state != hoPrepared || ho.target.conn != conn || ho.target.ENBUES1APID != notify.ENBUES1APID {
		m.mu.Unlock()
		logger.MmeLog.Warn("Handover Notify with no matching prepared handover", zap.Uint32("target-mme-ue-id", uint32(notify.MMEUES1APID)))

		return
	}

	// Committing locks out a concurrent CANCEL or the guard timer while the
	// user-plane switch I/O runs outside the lock.
	ho.state = hoCommitting
	admitted := ho.admitted
	releaseEBIs := ho.releaseEBIs
	m.mu.Unlock()

	// Switch the downlink only at notify (TS 23.401 §5.5.1.2.2 step 15).
	for _, a := range admitted {
		p := m.lookupPDN(ue, a.ebi)
		if p == nil {
			continue
		}

		if err := m.session.ModifyEPSSession(ctx, ue.imsi, a.ebi, a.enbFTEID); err != nil {
			logger.MmeLog.Error("failed to switch an EPS session downlink to the target eNB",
				zap.String("imsi", ue.imsi), zap.Uint8("e-rab-id", a.ebi), zap.Error(err))

			continue
		}

		ue.mu.Lock()
		p.enbFTEID = a.enbFTEID
		ue.mu.Unlock()
	}

	// Release the PDN connections whose default bearer the target rejected.
	for _, ebi := range releaseEBIs {
		if err := m.session.ReleaseEPSSession(ctx, ue.imsi, ebi); err != nil {
			logger.MmeLog.Error("failed to release a rejected PDN connection after handover",
				zap.String("imsi", ue.imsi), zap.Uint8("e-rab-id", ebi), zap.Error(err))
		}

		m.dropPDN(ue, ebi)
	}

	m.ensureDefaultPDN(ue, admitted)

	m.mu.Lock()
	source := ho.source
	ue.nh = ho.newNH
	ue.ncc = ho.newNCC
	ue.s1 = ho.target // the target becomes the UE's active connection
	source.ue = nil   // detach the source; its Release Complete removes the connection
	m.clearHandoverLocked(ue)
	targetMMEID := ue.s1.MMEUES1APID
	m.mu.Unlock()

	if notify.TAI.TAC != 0 {
		ue.touchLastSeen()
	}

	logger.MmeLog.Info("Handover Notify",
		zap.Uint32("target-mme-ue-id", uint32(targetMMEID)),
		zap.Uint32("target-enb-ue-id", uint32(notify.ENBUES1APID)))

	m.sendUEContextRelease(ctx, source.conn, source.MMEUES1APID, source.ENBUES1APID)
}

// handleHandoverCancel releases any prepared target resources and acknowledges,
// leaving the UE on the source (TS 36.413 §8.4.5). conn is the source.
func (m *MME) handleHandoverCancel(ctx context.Context, conn nasWriter, value []byte) {
	cancel, err := s1ap.ParseHandoverCancel(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Cancel", zap.Error(err))
		return
	}

	ue, ok := m.resolveUE(conn, cancel.MMEUES1APID, cancel.ENBUES1APID)
	if !ok {
		return
	}

	m.mu.Lock()

	var releaseTarget *s1Conn

	ho := ue.handover
	switch {
	case ho == nil:
		// Nothing to cancel; still acknowledge below (TS 36.413 §8.4.5.4).
	case ho.state == hoCommitting:
		// Too late to cancel: acknowledge but let the in-flight move finish.
	default:
		if ho.state == hoPrepared {
			releaseTarget = ho.target
		}

		m.clearHandoverLocked(ue)
	}
	m.mu.Unlock()

	if releaseTarget != nil {
		m.sendUEContextRelease(ctx, releaseTarget.conn, releaseTarget.MMEUES1APID, releaseTarget.ENBUES1APID)
	}

	ack := &s1ap.HandoverCancelAcknowledge{MMEUES1APID: cancel.MMEUES1APID, ENBUES1APID: cancel.ENBUES1APID}

	b, err := ack.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Handover Cancel Acknowledge", zap.Error(err))
		return
	}

	logger.MmeLog.Info("Handover Cancel", zap.Uint32("mme-ue-id", uint32(cancel.MMEUES1APID)))
	m.sendS1APConn(ctx, conn, S1APProcedureHandoverCancelAcknowledge, b)
}

// failHandoverToSource clears the handover and sends a HANDOVER PREPARATION
// FAILURE to the source eNB, leaving the UE on the source association. The target
// connection allocated for the preparation is dropped by clearHandoverLocked.
func (m *MME) failHandoverToSource(ctx context.Context, ue *UeContext, cause s1ap.Cause) {
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

	m.sendHandoverPreparationFailure(ctx, sourceConn, sourceMMEID, sourceENBID, cause)
}

// sendHandoverPreparationFailure sends a HANDOVER PREPARATION FAILURE on the
// source association (TS 36.413 §8.4.1.3).
func (m *MME) sendHandoverPreparationFailure(ctx context.Context, conn nasWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID, cause s1ap.Cause) {
	fail := &s1ap.HandoverPreparationFailure{MMEUES1APID: mmeUEID, ENBUES1APID: enbUEID, Cause: cause}

	b, err := fail.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal Handover Preparation Failure", zap.Error(err))
		return
	}

	m.sendS1APConn(ctx, conn, S1APProcedureHandoverPreparationFailure, b)
}

// sendUEContextRelease sends a UE Context Release Command for a handover
// association by its own MME-UE-S1AP-ID (TS 36.413 §8.4): the source after notify,
// or a rejected/superseded target. The connection is removed when its Release
// Complete arrives (releaseDetachedConn) or when its association drops.
func (m *MME) sendUEContextRelease(ctx context.Context, conn nasWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID) {
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
	m.sendS1APConn(ctx, conn, S1APProcedureUEContextReleaseCommand, b)
}

// releaseDetachedConn removes a UE-associated connection that holds no UE context —
// a handover source detached at HANDOVER NOTIFY, or a released target — when its UE
// Context Release Complete arrives, identified by its own MME-UE-S1AP-ID (TS 36.413
// §8.4). It reports whether it handled one.
func (m *MME) releaseDetachedConn(conn nasWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.conns[uint32(mmeUEID)]
	if !ok || c.ue != nil || c.conn != conn || c.ENBUES1APID != enbUEID {
		return false
	}

	delete(m.conns, uint32(mmeUEID))

	return true
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

	if ho.guardTimer != nil {
		ho.guardTimer.Stop()
	}

	if ho.target != nil && ho.target != ue.s1 {
		ho.target.ue = nil
		delete(m.conns, uint32(ho.target.MMEUES1APID))
	}

	ue.handover = nil
	ue.handoverGen++
	ue.keyChainBusy = false
}

// clearHandover drops the UE's in-flight handover context under MME.mu.
func (m *MME) clearHandover(ue *UeContext) {
	m.mu.Lock()
	m.clearHandoverLocked(ue)
	m.mu.Unlock()
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

	var releaseTarget *s1Conn
	if ho.state == hoPrepared {
		releaseTarget = ho.target
	}

	sourceMMEID := ho.source.MMEUES1APID

	m.clearHandoverLocked(ue)
	m.mu.Unlock()

	logger.MmeLog.Warn("S1 handover abandoned: target did not complete it in time",
		zap.Uint32("mme-ue-id", uint32(sourceMMEID)))

	if releaseTarget != nil {
		m.sendUEContextRelease(context.Background(), releaseTarget.conn, releaseTarget.MMEUES1APID, releaseTarget.ENBUES1APID)
	}
}

// ensureDefaultPDN promotes the lowest surviving admitted PDN to the UE's default
// when the attach-default PDN was released during a partial-admission handover, so
// a registered UE always retains a default PDN connection (its EPS last-resort
// connectivity, TS 23.401). A no-op when a default still exists or no admitted PDN
// survives.
func (m *MME) ensureDefaultPDN(ue *UeContext, admitted []admittedERAB) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.defaultEBI != 0 {
		return
	}

	lowest := uint8(0)

	for _, a := range admitted {
		if _, ok := ue.pdns[a.ebi]; !ok {
			continue
		}

		if lowest == 0 || a.ebi < lowest {
			lowest = a.ebi
		}
	}

	if lowest != 0 {
		ue.defaultEBI = lowest
	}
}

// handoverBearers snapshots the UE's PDN connections into the E-RABs To Be Setup
// list of a HANDOVER REQUEST (TS 36.413 §9.1.5.4): each bearer's serving GW S1-U
// uplink endpoint and QoS. It returns false when the UE has no usable bearer.
func (m *MME) handoverBearers(ue *UeContext) ([]s1ap.ERABToBeSetupItemHOReq, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	bearers := make([]s1ap.ERABToBeSetupItemHOReq, 0, len(ue.pdns))

	for _, p := range ue.pdns {
		sgwTLA, err := models.EncodeTransportLayerAddress(p.sgwFTEID.Addr, p.sgwN3IPv6)
		if err != nil {
			logger.MmeLog.Error("failed to encode S-GW transport layer address for handover",
				zap.String("imsi", ue.imsi), zap.Uint8("e-rab-id", p.ebi), zap.Error(err))

			continue
		}

		bearers = append(bearers, s1ap.ERABToBeSetupItemHOReq{
			ERABID:                s1ap.ERABID(p.ebi),
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(p.sgwFTEID.TEID),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(p.qci),
				ARP: s1ap.AllocationAndRetentionPriority{
					PriorityLevel:           p.arp,
					PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
					PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
				},
			},
		})
	}

	return bearers, len(bearers) > 0
}

// handoverUEAMBR builds the UE Aggregate Maximum Bit Rate IE for a HANDOVER
// REQUEST from the UE's stored profile UE-AMBR.
func (m *MME) handoverUEAMBR(ue *UeContext) s1ap.UEAggregateMaximumBitRate {
	return s1ap.UEAggregateMaximumBitRate{
		DL: s1ap.BitRate(bitRateToBps(ue.ambrDownlink)),
		UL: s1ap.BitRate(bitRateToBps(ue.ambrUplink)),
	}
}

// handoverSecurityCapabilities encodes the UE's stored security capabilities for a
// HANDOVER REQUEST.
func (m *MME) handoverSecurityCapabilities(ue *UeContext) s1ap.UESecurityCapabilities {
	uecap, err := eps.ParseUENetworkCapability(ue.ueNetCap)
	if err != nil {
		return s1ap.UESecurityCapabilities{}
	}

	return s1apSecurityCapabilities(uecap)
}

// failedHandoverEBIs returns the EPS bearer identities the target eNB did not
// admit: those it explicitly failed plus any the source offered that are missing
// from the admitted set.
func failedHandoverEBIs(ack *s1ap.HandoverRequestAcknowledge, admitted []admittedERAB) []uint8 {
	admittedSet := make(map[uint8]struct{}, len(admitted))
	for _, a := range admitted {
		admittedSet[a.ebi] = struct{}{}
	}

	seen := make(map[uint8]struct{})

	var out []uint8

	add := func(ebi uint8) {
		if _, ok := admittedSet[ebi]; ok {
			return
		}

		if _, ok := seen[ebi]; ok {
			return
		}

		seen[ebi] = struct{}{}
		out = append(out, ebi)
	}

	for _, it := range ack.ERABFailedToSetup {
		add(uint8(it.ERABID))
	}

	return out
}

// releaseItems renders EPS bearer identities as the E-RABs to Release List of a
// HANDOVER COMMAND (TS 36.413 §9.1.5.2).
func releaseItems(ebis []uint8) []s1ap.ERABItem {
	if len(ebis) == 0 {
		return nil
	}

	out := make([]s1ap.ERABItem, 0, len(ebis))
	for _, ebi := range ebis {
		out = append(out, s1ap.ERABItem{ERABID: s1ap.ERABID(ebi), Cause: causeHOFailureInTarget})
	}

	return out
}
