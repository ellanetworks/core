// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// targetGlobalENBID is the target eNB the handover scenarios route to; its
// formatted id matches the key registered in m.enbByID.
var targetGlobalENBID = s1ap.GlobalENBID{
	PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10},
	ENBID:        s1ap.ENBID{Kind: s1ap.ENBIDMacro, Value: 2},
}

// handoverUE builds a secured, registered UE on a source eNB with one PDN
// connection, and registers a target eNB association. It returns the UE, the
// source conn, and the target conn.
func handoverUE(t *testing.T, m *MME) (*UeContext, *captureConn, *captureConn) {
	t.Helper()

	ue, source := securedUE(t, m)
	p := testPDN(ue)
	p.apn = "internet"
	p.qci, p.arp = 9, 8
	p.sgwFTEID = models.FTEID{TEID: 0x1111, Addr: netip.AddrFrom4([4]byte{10, 0, 0, 1})}
	ue.ueNetCap = eps.UENetworkCapability{EEA: 0xe0, EIA: 0xe0}.Marshal()
	ue.ambrUplink, ue.ambrDownlink = "1 Gbps", "1 Gbps"
	ue.ncc = 1

	for i := range ue.nh {
		ue.nh[i] = byte(0x40 + i)
	}

	target := &captureConn{}

	m.mu.Lock()
	m.enbByID[enbID(targetGlobalENBID)] = target
	m.mu.Unlock()

	return ue, source, target
}

func sampleHandoverRequired(ue *UeContext) *s1ap.HandoverRequired {
	return &s1ap.HandoverRequired{
		MMEUES1APID:    ue.s1.MMEUES1APID,
		ENBUES1APID:    ue.s1.ENBUES1APID,
		HandoverType:   s1ap.HandoverTypeIntraLTE,
		Cause:          s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 16},
		TargetID:       s1ap.TargetID{TargeteNBID: s1ap.TargeteNBID{GlobalENBID: targetGlobalENBID, SelectedTAI: s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}}},
		SourceToTarget: s1ap.TransparentContainer{0x01, 0x02, 0x03},
	}
}

func mustMarshal(t *testing.T, marshal func() ([]byte, error)) []byte {
	t.Helper()

	b, err := marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func successfulValue(t *testing.T, b []byte) []byte {
	t.Helper()

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	return pdu.(*s1ap.SuccessfulOutcome).Value
}

func unsuccessfulValue(t *testing.T, b []byte) []byte {
	t.Helper()

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	return pdu.(*s1ap.UnsuccessfulOutcome).Value
}

// lastPDU unmarshals the most recently captured S1AP message on a conn, reading
// under the conn lock so it is safe against a concurrent timer-driven send.
func lastPDU(t *testing.T, cc *captureConn) s1ap.PDU {
	t.Helper()

	cc.mu.Lock()
	n := len(cc.sent)

	var raw []byte
	if n > 0 {
		raw = append([]byte(nil), cc.sent[n-1]...)
	}
	cc.mu.Unlock()

	if raw == nil {
		t.Fatal("no S1AP message captured")
	}

	pdu, err := s1ap.Unmarshal(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	return pdu
}

// driveToPrepared runs HANDOVER REQUIRED then HANDOVER REQUEST ACKNOWLEDGE so the
// handover reaches the prepared state, returning the target eNB-UE-S1AP-ID used.
func driveToPrepared(t *testing.T, m *MME, ue *UeContext, source, target *captureConn) s1ap.ENBUES1APID {
	t.Helper()

	m.handleHandoverRequired(context.Background(), source, initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if target.count() != 1 {
		t.Fatalf("expected one HANDOVER REQUEST to the target, got %d", target.count())
	}

	const targetENBUEID s1ap.ENBUES1APID = 55

	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: ue.s1.MMEUES1APID,
		ENBUES1APID: targetENBUEID,
		ERABAdmitted: []s1ap.ERABAdmittedItem{{
			ERABID:                s1ap.ERABID(defaultERABID),
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
		}},
		TargetToSource: s1ap.TransparentContainer{0xaa},
	}

	m.handleHandoverRequestAcknowledge(context.Background(), target, successfulValue(t, mustMarshal(t, ack.Marshal)))

	return targetENBUEID
}

// TestHandoverHappyPath drives the full S1 handover: REQUIRED → REQUEST →
// ACKNOWLEDGE → COMMAND → eNB STATUS TRANSFER → MME STATUS TRANSFER → NOTIFY,
// asserting the user plane switches to the target only at notify and the source is
// released.
func TestHandoverHappyPath(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	wantNH, err := deriveNH(ue.kasme, ue.nh[:])
	if err != nil {
		t.Fatal(err)
	}

	// HANDOVER REQUIRED → HANDOVER REQUEST to the target.
	m.handleHandoverRequired(context.Background(), source, initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	req, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || req.ProcedureCode != s1ap.ProcHandoverResourceAllocation {
		t.Fatalf("expected HANDOVER REQUEST to target, got %T", lastPDU(t, target))
	}

	hoReq, err := s1ap.ParseHandoverRequest(req.Value)
	if err != nil {
		t.Fatalf("parse HANDOVER REQUEST: %v", err)
	}

	if hoReq.MMEUES1APID != ue.s1.MMEUES1APID || len(hoReq.ERABToBeSetup) != 1 ||
		hoReq.ERABToBeSetup[0].GTPTEID != 0x1111 || hoReq.SecurityContext.NextHopChainingCount != 2 {
		t.Fatalf("HANDOVER REQUEST = %+v", hoReq)
	}

	if s1ap.SecurityKey(wantNH) != hoReq.SecurityContext.NextHopParameter {
		t.Fatal("HANDOVER REQUEST carried the wrong Next Hop")
	}

	// The user plane must NOT be switched during preparation.
	if fsm := m.session.(*fakeSessionManager); fsm.modifiedENB != (models.FTEID{}) {
		t.Fatalf("user plane switched during preparation: %+v", fsm.modifiedENB)
	}

	// HANDOVER REQUEST ACKNOWLEDGE → HANDOVER COMMAND to the source.
	const targetENBUEID s1ap.ENBUES1APID = 55

	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: ue.s1.MMEUES1APID,
		ENBUES1APID: targetENBUEID,
		ERABAdmitted: []s1ap.ERABAdmittedItem{{
			ERABID:                s1ap.ERABID(defaultERABID),
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
		}},
		TargetToSource: s1ap.TransparentContainer{0xaa},
	}

	m.handleHandoverRequestAcknowledge(context.Background(), target, successfulValue(t, mustMarshal(t, ack.Marshal)))

	cmd, ok := lastPDU(t, source).(*s1ap.SuccessfulOutcome)
	if !ok || cmd.ProcedureCode != s1ap.ProcHandoverPreparation {
		t.Fatalf("expected HANDOVER COMMAND to source, got %T", lastPDU(t, source))
	}

	// Still no user-plane switch before notify.
	if fsm := m.session.(*fakeSessionManager); fsm.modifiedENB != (models.FTEID{}) {
		t.Fatalf("user plane switched before notify: %+v", fsm.modifiedENB)
	}

	// eNB STATUS TRANSFER → MME STATUS TRANSFER to the target.
	st := &s1ap.ENBStatusTransfer{MMEUES1APID: ue.s1.MMEUES1APID, ENBUES1APID: ue.s1.ENBUES1APID, Container: s1ap.StatusTransferContainer{0xde, 0xad}}
	m.handleENBStatusTransfer(context.Background(), source, initiatingValue(t, mustMarshal(t, st.Marshal)))

	mst, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || mst.ProcedureCode != s1ap.ProcMMEStatusTransfer {
		t.Fatalf("expected MME STATUS TRANSFER to target, got %T", lastPDU(t, target))
	}

	parsedMST, err := s1ap.ParseMMEStatusTransfer(mst.Value)
	if err != nil || parsedMST.ENBUES1APID != targetENBUEID {
		t.Fatalf("MME STATUS TRANSFER = %+v, err %v", parsedMST, err)
	}

	// HANDOVER NOTIFY → user-plane switch, association move, source release.
	notify := &s1ap.HandoverNotify{
		MMEUES1APID: ue.s1.MMEUES1APID,
		ENBUES1APID: targetENBUEID,
		EUTRANCGI:   s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		TAI:         s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
	}
	m.handleHandoverNotify(context.Background(), target, initiatingValue(t, mustMarshal(t, notify.Marshal)))

	wantFTEID := models.FTEID{TEID: 0x99, Addr: netip.AddrFrom4([4]byte{10, 4, 0, 2})}
	if fsm := m.session.(*fakeSessionManager); fsm.modifiedENB != wantFTEID {
		t.Fatalf("ModifyEPSSession eNB F-TEID = %+v, want %+v", fsm.modifiedENB, wantFTEID)
	}

	if ue.s1.conn != target || ue.s1.ENBUES1APID != targetENBUEID || testPDN(ue).enbFTEID != wantFTEID {
		t.Fatalf("association not moved: conn=%v enb-id=%d", ue.s1.conn == target, ue.s1.ENBUES1APID)
	}

	if ue.ncc != 2 || ue.nh != wantNH {
		t.Fatalf("key chain not committed: ncc=%d nh-match=%v", ue.ncc, ue.nh == wantNH)
	}

	if ue.s1.handover != nil {
		t.Fatal("handover context not cleared after notify")
	}

	// The source eNB received a UE Context Release Command.
	rel, ok := lastPDU(t, source).(*s1ap.InitiatingMessage)
	if !ok || rel.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command to source, got %T", lastPDU(t, source))
	}

	// Its Release Complete is consumed without disturbing the moved UE.
	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.s1.MMEUES1APID, ENBUES1APID: ue.s1.ENBUES1APID}
	m.handleUEContextReleaseComplete(source, successfulValue(t, mustMarshal(t, complete.Marshal)))

	if _, ok := m.lookupUe(ue.s1.MMEUES1APID); !ok {
		t.Fatal("UE removed by the source Release Complete")
	}

	if ue.s1.conn != target {
		t.Fatal("UE association disturbed by the source Release Complete")
	}
}

// TestHandoverRequiredNoSecurityFails checks a UE without a security context is
// rejected with HANDOVER PREPARATION FAILURE and no HANDOVER REQUEST is sent.
func TestHandoverRequiredNoSecurityFails(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)
	ue.secured = false

	m.handleHandoverRequired(context.Background(), source, initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if target.count() != 0 {
		t.Fatalf("expected no HANDOVER REQUEST, got %d", target.count())
	}

	uo, ok := lastPDU(t, source).(*s1ap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != s1ap.ProcHandoverPreparation {
		t.Fatalf("expected HANDOVER PREPARATION FAILURE, got %T", lastPDU(t, source))
	}

	fail, _ := s1ap.ParseHandoverPreparationFailure(uo.Value)
	if fail.Cause != causeHandoverNoSecurity {
		t.Fatalf("cause = %+v, want authentication-failure", fail.Cause)
	}

	if ue.s1.handover != nil {
		t.Fatal("handover context left set on failure")
	}
}

// TestHandoverRequiredUnknownTargetFails checks an unresolvable target eNB yields
// HANDOVER PREPARATION FAILURE with cause unknown-targetID.
func TestHandoverRequiredUnknownTargetFails(t *testing.T) {
	m := newTestMME(t)
	ue, source, _ := handoverUE(t, m)

	req := sampleHandoverRequired(ue)
	req.TargetID.TargeteNBID.GlobalENBID.ENBID.Value = 999 // not registered

	m.handleHandoverRequired(context.Background(), source, initiatingValue(t, mustMarshal(t, req.Marshal)))

	uo, ok := lastPDU(t, source).(*s1ap.UnsuccessfulOutcome)
	if !ok {
		t.Fatalf("expected HANDOVER PREPARATION FAILURE, got %T", lastPDU(t, source))
	}

	fail, _ := s1ap.ParseHandoverPreparationFailure(uo.Value)
	if fail.Cause != causeUnknownTargetID {
		t.Fatalf("cause = %+v, want unknown-targetID", fail.Cause)
	}
}

// TestHandoverConcurrentRefused checks a second HANDOVER REQUIRED while a handover
// is in progress is rejected and does not disturb the first.
func TestHandoverConcurrentRefused(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	m.handleHandoverRequired(context.Background(), source, initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if ue.s1.handover == nil {
		t.Fatal("first handover did not start")
	}

	first := ue.s1.handover

	m.handleHandoverRequired(context.Background(), source, initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if ue.s1.handover != first {
		t.Fatal("second handover disturbed the in-flight one")
	}

	if target.count() != 1 {
		t.Fatalf("second HANDOVER REQUIRED sent another HANDOVER REQUEST: %d", target.count())
	}
}

// TestPathSwitchRefusedDuringHandover checks a Path Switch is refused while an S1
// handover is advancing the key chain, so the two cannot derive a fresh NH from
// the same base for different targets (TS 33.401 §7.2.8).
func TestPathSwitchRefusedDuringHandover(t *testing.T) {
	m := newTestMME(t)
	ue, source, _ := handoverUE(t, m)

	m.handleHandoverRequired(context.Background(), source, initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if ue.s1.handover == nil {
		t.Fatal("handover did not start")
	}

	ncc, nh, conn := ue.ncc, ue.nh, ue.s1.conn

	target := &captureConn{}
	m.handlePathSwitchRequest(context.Background(), target, pathSwitchValue(t, samplePathSwitchRequest(ue)))

	if target.count() != 1 {
		t.Fatalf("expected one downlink (Path Switch Failure), got %d", target.count())
	}

	parsePathSwitchFailure(t, target.sent[0])

	if ue.ncc != ncc || ue.nh != nh || ue.s1.conn != conn {
		t.Fatal("Path Switch advanced the key chain or moved the association during a handover")
	}
}

// TestHandoverRefusedWhileKeyChainBusy checks an S1 handover is refused while a
// Path Switch holds the key chain — the symmetric guard of the shared marker.
func TestHandoverRefusedWhileKeyChainBusy(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	m.mu.Lock()
	ue.keyChainBusy = true // a Path Switch is mid-advance
	m.mu.Unlock()

	m.handleHandoverRequired(context.Background(), source, initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if ue.s1.handover != nil {
		t.Fatal("handover started while the key chain was busy")
	}

	if target.count() != 0 {
		t.Fatalf("handover sent a HANDOVER REQUEST while the key chain was busy: %d", target.count())
	}

	uo, ok := lastPDU(t, source).(*s1ap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != s1ap.ProcHandoverPreparation {
		t.Fatalf("expected HANDOVER PREPARATION FAILURE to source, got %T", lastPDU(t, source))
	}
}

// TestHandoverFailureFailsToSource checks a HANDOVER FAILURE from the target ends
// the handover with a HANDOVER PREPARATION FAILURE to the source, the UE intact.
func TestHandoverFailureFailsToSource(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	m.handleHandoverRequired(context.Background(), source, initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	fail := &s1ap.HandoverFailure{MMEUES1APID: ue.s1.MMEUES1APID, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 12}}
	m.handleHandoverFailure(context.Background(), target, unsuccessfulValue(t, mustMarshal(t, fail.Marshal)))

	if ue.s1.handover != nil {
		t.Fatal("handover not cleared after failure")
	}

	uo, ok := lastPDU(t, source).(*s1ap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != s1ap.ProcHandoverPreparation {
		t.Fatalf("expected HANDOVER PREPARATION FAILURE to source, got %T", lastPDU(t, source))
	}

	if ue.s1.conn != source {
		t.Fatal("UE association moved on a failed handover")
	}
}

// TestHandoverCancelReleasesTarget checks a HANDOVER CANCEL after preparation
// releases the target context and acknowledges, leaving the UE on the source.
func TestHandoverCancelReleasesTarget(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetENBUEID := driveToPrepared(t, m, ue, source, target)

	cancel := &s1ap.HandoverCancel{MMEUES1APID: ue.s1.MMEUES1APID, ENBUES1APID: ue.s1.ENBUES1APID, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5}}
	m.handleHandoverCancel(context.Background(), source, initiatingValue(t, mustMarshal(t, cancel.Marshal)))

	if ue.s1.handover != nil {
		t.Fatal("handover not cleared after cancel")
	}

	// The target received a UE Context Release Command.
	trel, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || trel.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command to target, got %T", lastPDU(t, target))
	}

	// The source received a HANDOVER CANCEL ACKNOWLEDGE.
	ack, ok := lastPDU(t, source).(*s1ap.SuccessfulOutcome)
	if !ok || ack.ProcedureCode != s1ap.ProcHandoverCancel {
		t.Fatalf("expected HANDOVER CANCEL ACKNOWLEDGE to source, got %T", lastPDU(t, source))
	}

	if ue.s1.conn != source {
		t.Fatal("UE association moved on a cancelled handover")
	}

	// The target's Release Complete is consumed.
	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: ue.s1.MMEUES1APID, ENBUES1APID: targetENBUEID}
	m.handleUEContextReleaseComplete(target, successfulValue(t, mustMarshal(t, complete.Marshal)))

	if _, ok := m.lookupUe(ue.s1.MMEUES1APID); !ok {
		t.Fatal("UE removed by the target Release Complete")
	}
}

// TestHandoverPartialAdmissionReleasesFailedPDN checks a multi-PDN UE whose target
// rejects one PDN's default bearer keeps the admitted PDN and releases the
// rejected one at notify (TS 23.401 §5.5.1.2.2 step 15).
func TestHandoverPartialAdmissionReleasesFailedPDN(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	// Second PDN connection on EBI 6.
	second := ue.ensurePDN(6)
	second.apn = "ims"
	second.qci, second.arp = 5, 7
	second.sgwFTEID = models.FTEID{TEID: 0x2222, Addr: netip.AddrFrom4([4]byte{10, 0, 0, 2})}

	m.handleHandoverRequired(context.Background(), source, initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	req, _ := s1ap.ParseHandoverRequest(lastPDU(t, target).(*s1ap.InitiatingMessage).Value)
	if len(req.ERABToBeSetup) != 2 {
		t.Fatalf("expected 2 E-RABs in HANDOVER REQUEST, got %d", len(req.ERABToBeSetup))
	}

	const targetENBUEID s1ap.ENBUES1APID = 55

	// Target admits the default bearer (EBI 5), rejects EBI 6.
	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: ue.s1.MMEUES1APID,
		ENBUES1APID: targetENBUEID,
		ERABAdmitted: []s1ap.ERABAdmittedItem{{
			ERABID:                s1ap.ERABID(defaultERABID),
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
		}},
		ERABFailedToSetup: []s1ap.ERABItem{{ERABID: 6, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}}},
		TargetToSource:    s1ap.TransparentContainer{0xaa},
	}
	m.handleHandoverRequestAcknowledge(context.Background(), target, successfulValue(t, mustMarshal(t, ack.Marshal)))

	// HANDOVER COMMAND lists EBI 6 in the bearers-to-release list.
	cmd, _ := s1ap.ParseHandoverCommand(lastPDU(t, source).(*s1ap.SuccessfulOutcome).Value)
	if len(cmd.ERABToRelease) != 1 || cmd.ERABToRelease[0].ERABID != 6 {
		t.Fatalf("HANDOVER COMMAND release list = %+v", cmd.ERABToRelease)
	}

	notify := &s1ap.HandoverNotify{
		MMEUES1APID: ue.s1.MMEUES1APID,
		ENBUES1APID: targetENBUEID,
		EUTRANCGI:   s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		TAI:         s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
	}
	m.handleHandoverNotify(context.Background(), target, initiatingValue(t, mustMarshal(t, notify.Marshal)))

	// The rejected PDN was released and dropped; the admitted one survives.
	if fsm := m.session.(*fakeSessionManager); !fsm.released {
		t.Fatal("rejected PDN session not released")
	}

	if m.lookupPDN(ue, 6) != nil {
		t.Fatal("rejected PDN connection not dropped")
	}

	if m.lookupPDN(ue, defaultERABID) == nil {
		t.Fatal("admitted PDN connection dropped")
	}
}

// TestHandoverCancelDuringCommitIgnored checks a HANDOVER CANCEL that races in
// after the UE has reached the target (the committing window) is acknowledged but
// does not tear the handover down — the fix for the NOTIFY-vs-CANCEL race.
func TestHandoverCancelDuringCommitIgnored(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	driveToPrepared(t, m, ue, source, target)

	// Simulate the in-flight user-plane switch of HANDOVER NOTIFY.
	m.mu.Lock()
	ue.s1.handover.state = hoCommitting
	m.mu.Unlock()

	targetBefore := target.count()

	cancel := &s1ap.HandoverCancel{MMEUES1APID: ue.s1.MMEUES1APID, ENBUES1APID: ue.s1.ENBUES1APID, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5}}
	m.handleHandoverCancel(context.Background(), source, initiatingValue(t, mustMarshal(t, cancel.Marshal)))

	if ue.s1.handover == nil {
		t.Fatal("a committing handover was torn down by a late cancel")
	}

	if target.count() != targetBefore {
		t.Fatal("a late cancel released the target during the commit window")
	}

	ack, ok := lastPDU(t, source).(*s1ap.SuccessfulOutcome)
	if !ok || ack.ProcedureCode != s1ap.ProcHandoverCancel {
		t.Fatalf("expected HANDOVER CANCEL ACKNOWLEDGE, got %T", lastPDU(t, source))
	}
}

// TestHandoverGuardTimerAbandons checks the supervision timer abandons a prepared
// handover the target never completes, releasing the target and leaving the UE on
// the source eNB.
func TestHandoverGuardTimerAbandons(t *testing.T) {
	m := newTestMME(t)
	m.handoverGuardTimeout = 50 * time.Millisecond

	ue, source, target := handoverUE(t, m)

	driveToPrepared(t, m, ue, source, target)

	// The guard expiry clears the handover and then releases the target, so the
	// target receives a second message (after the HANDOVER REQUEST).
	deadline := time.Now().Add(2 * time.Second)
	for target.count() < 2 {
		if time.Now().After(deadline) {
			t.Fatal("guard timer did not release the target")
		}

		time.Sleep(10 * time.Millisecond)
	}

	m.mu.RLock()

	cleared := ue.s1.handover == nil
	conn := ue.s1.conn

	m.mu.RUnlock()

	if !cleared {
		t.Fatal("handover not cleared after guard expiry")
	}

	if conn != source {
		t.Fatal("UE association moved when the handover was abandoned")
	}

	rel, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || rel.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command to target, got %T", lastPDU(t, target))
	}
}

// TestHandoverPartialAdmissionPromotesDefault checks that when the target rejects
// the UE's attach-default PDN but admits a secondary, the surviving PDN is promoted
// to the default so the UE retains a default PDN connection (TS 23.401 §5.5.1.2.2).
func TestHandoverPartialAdmissionPromotesDefault(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	second := ue.ensurePDN(6)
	second.apn = "ims"
	second.qci, second.arp = 5, 7
	second.sgwFTEID = models.FTEID{TEID: 0x2222, Addr: netip.AddrFrom4([4]byte{10, 0, 0, 2})}

	m.handleHandoverRequired(context.Background(), source, initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	const targetENBUEID s1ap.ENBUES1APID = 55

	// Target admits the secondary (EBI 6), rejects the attach default (EBI 5).
	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: ue.s1.MMEUES1APID,
		ENBUES1APID: targetENBUEID,
		ERABAdmitted: []s1ap.ERABAdmittedItem{{
			ERABID:                6,
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
		}},
		ERABFailedToSetup: []s1ap.ERABItem{{ERABID: s1ap.ERABID(defaultERABID), Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}}},
		TargetToSource:    s1ap.TransparentContainer{0xaa},
	}
	m.handleHandoverRequestAcknowledge(context.Background(), target, successfulValue(t, mustMarshal(t, ack.Marshal)))

	notify := &s1ap.HandoverNotify{
		MMEUES1APID: ue.s1.MMEUES1APID,
		ENBUES1APID: targetENBUEID,
		EUTRANCGI:   s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		TAI:         s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
	}
	m.handleHandoverNotify(context.Background(), target, initiatingValue(t, mustMarshal(t, notify.Marshal)))

	if m.lookupPDN(ue, defaultERABID) != nil {
		t.Fatal("rejected attach-default PDN not dropped")
	}

	ue.mu.Lock()
	defaultEBI := ue.defaultEBI
	ue.mu.Unlock()

	if defaultEBI != 6 {
		t.Fatalf("default EBI = %d, want 6 (promoted survivor)", defaultEBI)
	}
}
