// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
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

// admittedERAB is one E-RAB the target eNB admitted: the EPS bearer identity and
// the target eNB's S1-U downlink endpoint the user plane is switched to at notify.
type admittedERAB struct {
	ebi      uint8
	enbFTEID models.FTEID
}

// handoverContext is the MME's state for one in-flight inter-eNB S1 handover
// (TS 36.413 §8.4, TS 23.401 §5.5.1.2.2). The MME keeps the same MME-UE-S1AP-ID
// across the handover; the target eNB allocates its own eNB-UE-S1AP-ID, learned
// from the HANDOVER REQUEST ACKNOWLEDGE. Guarded by MME.mu.
type handoverContext struct {
	state         hoState
	sourceConn    nasWriter
	sourceENBUEID s1ap.ENBUES1APID
	target        nasWriter
	targetENBUEID s1ap.ENBUES1APID
	admitted      []admittedERAB
	releaseEBIs   []uint8 // EPS bearers the target rejected, released at notify (per-PDN, §5.5.1.2.2 step 15)
	// newNH/newNCC are the {NH, NCC} computed at preparation, handed to the target
	// in the HANDOVER REQUEST and committed at notify (TS 33.401 §7.2.8).
	newNH  [32]byte
	newNCC uint8
	// guardTimer bounds the whole procedure (HANDOVER REQUIRED → NOTIFY): on expiry
	// the MME abandons the handover and releases any prepared target resources, so a
	// target that goes silent does not pin the UE's handover slot forever.
	guardTimer *time.Timer
}

// srcReleaseKey identifies a UE Context Release Command the MME sent to a source
// (or rejected target) eNB during a handover, so its Release Complete is consumed
// without disturbing the UE now active on the target association.
//
// Intra-MME S1 handover keeps one UeContext keyed by a single MME-UE-S1AP-ID
// across both associations, so unlike the AMF — which models distinct source and
// target RAN contexts under one UE — the 4G MME cannot tell the source's Release
// Complete apart from the moved UE's by id alone. This {conn, eNB-UE-S1AP-ID} set
// is the side-channel that disambiguates them. It is bounded: entries are removed
// when the Release Complete arrives, and abortHandoversOnConnLoss sweeps any left
// behind when an association drops. A source/target RAN-context split would be the
// structurally cleaner model but is unnecessary for the single-node, intra-MME
// topology Ella Core targets.
type srcReleaseKey struct {
	conn    nasWriter
	enbUEID s1ap.ENBUES1APID
}

// sendS1APConn sends an S1AP message on a specific eNB association (not the UE's
// current conn), used during handover when source and target are different
// associations.
func (m *MME) sendS1APConn(ctx context.Context, conn nasWriter, messageType S1APProcedure, b []byte) {
	if _, err := conn.WriteMsg(b, &sctp.SndRcvInfo{PPID: s1apWirePPID, Stream: s1apStreamUE}); err != nil {
		logger.MmeLog.Error("failed to send S1AP message", zap.String("message-type", string(messageType)), zap.Error(err))
		return
	}

	m.logOutboundS1AP(ctx, conn, messageType, b)
}

// handleHandoverRequired handles a HANDOVER REQUIRED from the source eNB
// (TS 36.413 §8.4.1, TS 23.401 §5.5.1.2.2 step 2): it resolves the target eNB,
// reserves the {NH, NCC} for it, and sends a HANDOVER REQUEST — or replies
// HANDOVER PREPARATION FAILURE. conn is the source eNB association.
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

	// Advance the {NH, NCC} for the target before any commitment; the chain is
	// committed only at notify (TS 33.401 §7.2.8).
	newNH, err := deriveNH(ue.kasme, ue.nh[:])
	if err != nil {
		logger.MmeLog.Error("failed to advance NH for handover", zap.Error(err))
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)

		return
	}

	newNCC := (ue.ncc + 1) & 0x07

	// Only one handover preparation may be ongoing for a UE (TS 36.413 §8.4.1.1).
	m.mu.Lock()
	if ue.handover != nil {
		m.mu.Unlock()
		logger.MmeLog.Warn("Handover Required while a handover is already in progress",
			zap.Uint32("mme-ue-id", uint32(req.MMEUES1APID)))
		m.sendHandoverPreparationFailure(ctx, conn, req.MMEUES1APID, req.ENBUES1APID, causeHandoverPrepUnspecific)

		return
	}

	gen := ue.handoverGen
	ho := &handoverContext{
		state:         hoPreparing,
		sourceConn:    conn,
		sourceENBUEID: req.ENBUES1APID,
		target:        target,
		newNH:         newNH,
		newNCC:        newNCC,
	}
	ho.guardTimer = time.AfterFunc(m.handoverGuardTimeout, func() { m.onHandoverGuardExpiry(ue, gen) })
	ue.handover = ho
	mmeUEID := ue.MMEUES1APID
	m.mu.Unlock()

	hoReq := &s1ap.HandoverRequest{
		MMEUES1APID:            mmeUEID,
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
		zap.Uint32("mme-ue-id", uint32(mmeUEID)),
		zap.String("target-enb", enbID(req.TargetID.TargeteNBID.GlobalENBID)),
		zap.Int("e-rabs", len(bearers)))
	m.sendS1APConn(ctx, target, S1APProcedureHandoverRequest, b)
}

// handleHandoverRequestAcknowledge handles a HANDOVER REQUEST ACKNOWLEDGE from the
// target eNB (TS 36.413 §8.4.2, TS 23.401 §5.5.1.2.2 step 5a): it records the
// target's downlink endpoints and sends a HANDOVER COMMAND to the source — or
// fails the handover when the target admitted no usable bearer. conn is the target
// eNB association.
func (m *MME) handleHandoverRequestAcknowledge(ctx context.Context, conn nasWriter, value []byte) {
	ack, err := s1ap.ParseHandoverRequestAcknowledge(value)
	if err != nil {
		logger.MmeLog.Warn("failed to decode Handover Request Acknowledge", zap.Error(err))
		return
	}

	ue, ok := m.lookupUe(ack.MMEUES1APID)
	if !ok {
		// No UE for this id; release the target context the ack just created.
		m.releaseHandoverTarget(ctx, conn, ack.MMEUES1APID, ack.ENBUES1APID)
		return
	}

	m.mu.Lock()
	ho := ue.handover

	if ho == nil || ho.state != hoPreparing || ho.target != conn {
		m.mu.Unlock()
		logger.MmeLog.Warn("Handover Request Acknowledge with no matching preparation",
			zap.Uint32("mme-ue-id", uint32(ack.MMEUES1APID)))
		m.releaseHandoverTarget(ctx, conn, ack.MMEUES1APID, ack.ENBUES1APID)

		return
	}

	ho.targetENBUEID = ack.ENBUES1APID
	m.mu.Unlock()

	admitted := make([]admittedERAB, 0, len(ack.ERABAdmitted))

	for _, it := range ack.ERABAdmitted {
		addr, ok := enbTransportAddress(it.TransportLayerAddress)
		if !ok {
			logger.MmeLog.Warn("Handover Request Acknowledge E-RAB has an invalid target address; treating as failed",
				zap.Uint32("mme-ue-id", uint32(ack.MMEUES1APID)), zap.Uint8("e-rab-id", uint8(it.ERABID)))

			continue
		}

		admitted = append(admitted, admittedERAB{ebi: uint8(it.ERABID), enbFTEID: models.FTEID{TEID: uint32(it.GTPTEID), Addr: addr}})
	}

	// A failed E-RAB is a failed PDN connection: 4G has one default bearer per PDN,
	// so a rejected bearer means that PDN must be released (TS 23.401 §5.5.1.2.2
	// step 15). Bearers neither admitted nor explicitly failed are also released.
	releaseEBIs := failedHandoverEBIs(ack, admitted)

	if len(admitted) == 0 {
		// No default bearer admitted: the handover is rejected (TS 23.401 §5.5.1.2.3).
		logger.MmeLog.Warn("Handover Request Acknowledge admitted no E-RAB; rejecting handover",
			zap.Uint32("mme-ue-id", uint32(ack.MMEUES1APID)))
		m.releaseHandoverTarget(ctx, conn, ack.MMEUES1APID, ack.ENBUES1APID)
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
	sourceConn, sourceENBUEID := ho.sourceConn, ho.sourceENBUEID
	mmeUEID := ue.MMEUES1APID
	m.mu.Unlock()

	cmd := &s1ap.HandoverCommand{
		MMEUES1APID:    mmeUEID,
		ENBUES1APID:    sourceENBUEID,
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
		zap.Uint32("mme-ue-id", uint32(mmeUEID)),
		zap.Int("admitted", len(admitted)),
		zap.Int("released", len(releaseEBIs)))
	m.sendS1APConn(ctx, sourceConn, S1APProcedureHandoverCommand, b)
}

// handleHandoverFailure handles a HANDOVER FAILURE from the target eNB (TS 36.413
// §8.4.2.3): the target could not admit the handover, so the MME fails the
// preparation toward the source and leaves the UE on the source eNB. conn is the
// target eNB association.
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

	if ho == nil || ho.target != conn {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	logger.MmeLog.Info("Handover Failure", zap.Uint32("mme-ue-id", uint32(fail.MMEUES1APID)))
	m.failHandoverToSource(ctx, ue, causeHOFailureInTarget)
}

// handleENBStatusTransfer relays an ENB STATUS TRANSFER from the source eNB to the
// target as an MME STATUS TRANSFER (TS 36.413 §8.4.6/§8.4.7). The container is
// relayed opaquely. It is a no-op when no handover is prepared for the UE; the
// source eNB may also omit this message entirely, so it never gates completion.
// conn is the source eNB association.
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

	if ho == nil || ho.target == nil {
		m.mu.Unlock()
		logger.MmeLog.Warn("eNB Status Transfer with no handover in progress", zap.Uint32("mme-ue-id", uint32(st.MMEUES1APID)))

		return
	}

	target, targetENBUEID := ho.target, ho.targetENBUEID
	mmeUEID := ue.MMEUES1APID
	m.mu.Unlock()

	mst := &s1ap.MMEStatusTransfer{MMEUES1APID: mmeUEID, ENBUES1APID: targetENBUEID, Container: st.Container}

	b, err := mst.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal MME Status Transfer", zap.Error(err))
		return
	}

	m.sendS1APConn(ctx, target, S1APProcedureMMEStatusTransfer, b)
}

// handleHandoverNotify handles a HANDOVER NOTIFY from the target eNB (TS 36.413
// §8.4.3, TS 23.401 §5.5.1.2.2 step 13): the UE has arrived, so the MME switches
// the user plane downlink to the target, commits the {NH, NCC} chain, moves the S1
// association, and releases the source eNB. conn is the target eNB association.
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

	if ho == nil || ho.state != hoPrepared || ho.target != conn || ho.targetENBUEID != notify.ENBUES1APID {
		m.mu.Unlock()
		logger.MmeLog.Warn("Handover Notify with no matching prepared handover", zap.Uint32("mme-ue-id", uint32(notify.MMEUES1APID)))

		return
	}

	// The UE has reached the target: enter the committing state so a concurrent
	// HANDOVER CANCEL (or the guard timer) on the source association cannot tear
	// the handover down while the user-plane switch is in flight.
	ho.state = hoCommitting
	m.mu.Unlock()

	// The user plane is switched only now (TS 23.401 §5.5.1.2.2 step 15): point each
	// admitted E-RAB's downlink at the target eNB endpoint.
	for _, a := range ho.admitted {
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
	for _, ebi := range ho.releaseEBIs {
		if err := m.session.ReleaseEPSSession(ctx, ue.imsi, ebi); err != nil {
			logger.MmeLog.Error("failed to release a rejected PDN connection after handover",
				zap.String("imsi", ue.imsi), zap.Uint8("e-rab-id", ebi), zap.Error(err))
		}

		m.dropPDN(ue, ebi)
	}

	// Keep a coherent default PDN: if the bearer dropped above was the attach
	// default while admitted PDNs remain, promote a survivor so the UE still has a
	// default PDN connection (TS 23.401 §5.5.1.2.2 step 15 releases the PDN, not
	// the UE's last-resort connectivity).
	m.ensureDefaultPDN(ue, ho.admitted)

	// Commit the advanced {NH, NCC} and move the S1 association to the target.
	m.mu.Lock()
	source := srcReleaseKey{conn: ho.sourceConn, enbUEID: ho.sourceENBUEID}
	ue.nh = ho.newNH
	ue.ncc = ho.newNCC
	ue.conn = conn
	ue.ENBUES1APID = notify.ENBUES1APID
	m.clearHandoverLocked(ue)
	m.handoverSrcReleases[source] = struct{}{}
	mmeUEID := ue.MMEUES1APID
	m.mu.Unlock()

	if notify.TAI.TAC != 0 {
		ue.touchLastSeen()
	}

	logger.MmeLog.Info("Handover Notify",
		zap.Uint32("mme-ue-id", uint32(mmeUEID)),
		zap.Uint32("target-enb-ue-id", uint32(notify.ENBUES1APID)))

	// Release the source eNB's UE context (TS 23.401 §5.5.1.2.2 step 19). The
	// command is addressed to the source association and its Release Complete is
	// consumed by handleUEContextReleaseComplete without touching the moved UE.
	m.sendSourceRelease(ctx, source, mmeUEID)
}

// handleHandoverCancel handles a HANDOVER CANCEL from the source eNB (TS 36.413
// §8.4.5, TS 23.401 §5.5.1.2.4): it releases any prepared target resources and
// acknowledges, leaving the UE on the source eNB. conn is the source eNB
// association.
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

	var (
		target        nasWriter
		targetENBUEID s1ap.ENBUES1APID
		hadTarget     bool
	)

	ho := ue.handover
	switch {
	case ho == nil:
		// Nothing to cancel; still acknowledge below (TS 36.413 §8.4.5.4).
	case ho.state == hoCommitting:
		// The UE has already reached the target and the move is in progress; it is
		// too late to cancel. Acknowledge but leave the handover to finish.
	default:
		if ho.state == hoPrepared {
			target, targetENBUEID, hadTarget = ho.target, ho.targetENBUEID, true
		}

		m.clearHandoverLocked(ue)
	}
	m.mu.Unlock()

	if hadTarget {
		m.releaseHandoverTarget(ctx, target, cancel.MMEUES1APID, targetENBUEID)
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
// FAILURE to the source eNB, leaving the UE on the source association.
func (m *MME) failHandoverToSource(ctx context.Context, ue *UeContext, cause s1ap.Cause) {
	m.mu.Lock()
	ho := ue.handover

	if ho == nil {
		m.mu.Unlock()
		return
	}

	sourceConn, sourceENBUEID := ho.sourceConn, ho.sourceENBUEID
	mmeUEID := ue.MMEUES1APID
	m.clearHandoverLocked(ue)
	m.mu.Unlock()

	m.sendHandoverPreparationFailure(ctx, sourceConn, mmeUEID, sourceENBUEID, cause)
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

// releaseHandoverTarget releases a target eNB's half-prepared UE context with a
// UE Context Release Command (TS 36.413), tracking the release so its Complete is
// consumed cleanly.
func (m *MME) releaseHandoverTarget(ctx context.Context, conn nasWriter, mmeUEID s1ap.MMEUES1APID, enbUEID s1ap.ENBUES1APID) {
	key := srcReleaseKey{conn: conn, enbUEID: enbUEID}

	m.mu.Lock()
	m.handoverSrcReleases[key] = struct{}{}
	m.mu.Unlock()

	cmd := &s1ap.UEContextReleaseCommand{
		UES1APIDs: s1ap.UES1APIDs{MMEUES1APID: mmeUEID, ENBUES1APID: enbUEID, Pair: true},
		Cause:     causeSuccessfulHandover,
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal target UE Context Release Command", zap.Error(err))
		return
	}

	m.sendS1APConn(ctx, conn, S1APProcedureUEContextReleaseCommand, b)
}

// sendSourceRelease sends the source eNB its UE Context Release Command after a
// completed handover (TS 23.401 §5.5.1.2.2 step 19).
func (m *MME) sendSourceRelease(ctx context.Context, key srcReleaseKey, mmeUEID s1ap.MMEUES1APID) {
	cmd := &s1ap.UEContextReleaseCommand{
		UES1APIDs: s1ap.UES1APIDs{MMEUES1APID: mmeUEID, ENBUES1APID: key.enbUEID, Pair: true},
		Cause:     causeSuccessfulHandover,
	}

	b, err := cmd.Marshal()
	if err != nil {
		logger.MmeLog.Error("failed to marshal source UE Context Release Command", zap.Error(err))
		return
	}

	logger.MmeLog.Info("UE Context Release Command (handover source)", zap.Uint32("mme-ue-id", uint32(mmeUEID)))
	m.sendS1APConn(ctx, key.conn, S1APProcedureUEContextReleaseCommand, b)
}

// consumeHandoverRelease reports whether a UE Context Release Complete on conn for
// enbUEID acknowledges a handover-related release the MME initiated; if so it
// removes the tracking entry so the complete is not applied to the moved UE.
func (m *MME) consumeHandoverRelease(conn nasWriter, enbUEID s1ap.ENBUES1APID) bool {
	key := srcReleaseKey{conn: conn, enbUEID: enbUEID}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.handoverSrcReleases[key]; ok {
		delete(m.handoverSrcReleases, key)
		return true
	}

	return false
}

// clearHandoverLocked drops the UE's in-flight handover context, stops its guard
// timer, and bumps handoverGen so a guard callback that fired concurrently is
// recognised as stale. The caller holds MME.mu.
func (m *MME) clearHandoverLocked(ue *UeContext) {
	if ue.handover == nil {
		return
	}

	if ue.handover.guardTimer != nil {
		ue.handover.guardTimer.Stop()
	}

	ue.handover = nil
	ue.handoverGen++
}

// clearHandover drops the UE's in-flight handover context under MME.mu.
func (m *MME) clearHandover(ue *UeContext) {
	m.mu.Lock()
	m.clearHandoverLocked(ue)
	m.mu.Unlock()
}

// onHandoverGuardExpiry abandons a handover whose target never completed it
// (TS 36.413 §8.4): the UE stays on the source eNB and any prepared target
// resources are released. gen guards against a callback that fired just as the
// handover was cleared or replaced. A handover already committing (the UE has
// reached the target) is left to finish.
func (m *MME) onHandoverGuardExpiry(ue *UeContext, gen uint64) {
	m.mu.Lock()

	ho := ue.handover
	if ho == nil || ue.handoverGen != gen || ho.state == hoCommitting {
		m.mu.Unlock()
		return
	}

	target, targetENBUEID, prepared := ho.target, ho.targetENBUEID, ho.state == hoPrepared
	mmeUEID := ue.MMEUES1APID
	m.clearHandoverLocked(ue)
	m.mu.Unlock()

	logger.MmeLog.Warn("S1 handover abandoned: target did not complete it in time",
		zap.Uint32("mme-ue-id", uint32(mmeUEID)))

	// A prepared target allocated a UE context; release it. A still-preparing
	// target either never answered or its later acknowledge will self-release on
	// finding no matching preparation.
	if prepared && target != nil {
		m.releaseHandoverTarget(context.Background(), target, mmeUEID, targetENBUEID)
	}
}

// handoverInProgress reports whether the UE has an in-flight S1 handover, so the
// data-network reconciler defers E-RAB management during the handover window
// (TS 36.413 §8.4.1.2).
func (m *MME) handoverInProgress(ue *UeContext) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ue.handover != nil
}

// abortHandoversOnConnLoss clears any in-flight handover that referenced a dropped
// eNB association, so the context does not leak when an SCTP association closes
// mid-handover.
func (m *MME) abortHandoversOnConnLoss(conn nasWriter) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for k := range m.handoverSrcReleases {
		if k.conn == conn {
			delete(m.handoverSrcReleases, k)
		}
	}

	for _, ue := range m.ues {
		if ho := ue.handover; ho != nil && (ho.sourceConn == conn || ho.target == conn) {
			m.clearHandoverLocked(ue)
		}
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
// HANDOVER REQUEST, matching the Initial Context Setup encoding (the S1AP encoding
// drops the EEA0/EIA0 bit, so the UE network capability octet is shifted left).
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
