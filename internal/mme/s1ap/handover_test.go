// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// targetGlobalENBID is the target eNB the handover scenarios route to; its
// formatted id matches the key registered in m.radiosByID.
var targetGlobalENBID = s1ap.GlobalENBID{
	PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10},
	ENBID:        s1ap.ENBID{Kind: s1ap.ENBIDMacro, Value: 2},
}

// handoverUE builds a secured, registered UE on a source eNB with one PDN
// connection, and registers a target eNB association. It returns the UE, the
// source conn, and the target conn.
func handoverUE(t *testing.T, m *mme.MME) (*mme.UeContext, *captureConn, *captureConn) {
	t.Helper()

	ue, source := securedUE(t, m)
	p := testPDN(ue)
	p.Apn = "internet"
	p.Qci, p.Arp = 9, 8
	p.SgwFTEID = models.FTEID{TEID: 0x1111, Addr: netip.AddrFrom4([4]byte{10, 0, 0, 1})}
	ue.UeNetCap = eps.UENetworkCapability{EEA: 0xe0, EIA: 0xe0}.Marshal()
	ue.Ambr = &models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"}
	ue.SetNCCForTest(1)

	var nh [32]byte
	for i := range nh {
		nh[i] = byte(0x40 + i)
	}

	ue.SetNHForTest(nh)

	target := &captureConn{}

	m.RegisterENBByIDForTest(targetGlobalENBID, target)

	return ue, source, target
}

func sampleHandoverRequired(ue *mme.UeContext) *s1ap.HandoverRequired {
	return &s1ap.HandoverRequired{
		MMEUES1APID:    ue.Conn().MMEUES1APID,
		ENBUES1APID:    ue.Conn().ENBUES1APID,
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

// targetMMEUEID reads the MME-UE-S1AP-ID the MME assigned to the target connection
// from the HANDOVER REQUEST it sent — the target's own id, distinct from the
// source's (TS 36.413).
func targetMMEUEID(t *testing.T, target *captureConn) s1ap.MMEUES1APID {
	t.Helper()

	req, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || req.ProcedureCode != s1ap.ProcHandoverResourceAllocation {
		t.Fatalf("expected HANDOVER REQUEST to target, got %T", lastPDU(t, target))
	}

	hoReq, err := s1ap.ParseHandoverRequest(req.Value)
	if err != nil {
		t.Fatalf("parse HANDOVER REQUEST: %v", err)
	}

	return hoReq.MMEUES1APID
}

// driveToPrepared runs HANDOVER REQUIRED then HANDOVER REQUEST ACKNOWLEDGE so the
// handover reaches the prepared state, returning the target's MME-UE-S1AP-ID and
// eNB-UE-S1AP-ID.
func driveToPrepared(t *testing.T, m *mme.MME, ue *mme.UeContext, source, target *captureConn) (s1ap.MMEUES1APID, s1ap.ENBUES1APID) {
	t.Helper()

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if target.count() != 1 {
		t.Fatalf("expected one HANDOVER REQUEST to the target, got %d", target.count())
	}

	targetMME := targetMMEUEID(t, target)

	const targetENBUEID s1ap.ENBUES1APID = 55

	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENBUEID,
		ERABAdmitted: []s1ap.ERABAdmittedItem{{
			ERABID:                s1ap.ERABID(mme.DefaultERABID),
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
		}},
		TargetToSource: s1ap.TransparentContainer{0xaa},
	}

	handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(target), successfulValue(t, mustMarshal(t, ack.Marshal)))

	return targetMME, targetENBUEID
}

// TestHandoverHappyPath drives the full S1 handover: REQUIRED → REQUEST →
// ACKNOWLEDGE → COMMAND → eNB STATUS TRANSFER → MME STATUS TRANSFER → NOTIFY,
// asserting the user plane switches to the target only at notify and the source is
// released.
func TestHandoverHappyPath(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	sourceMME := ue.Conn().MMEUES1APID
	sourceENB := ue.Conn().ENBUES1APID

	wantNH, err := ue.DeriveNextNHForTest()
	if err != nil {
		t.Fatal(err)
	}

	// HANDOVER REQUIRED → HANDOVER REQUEST to the target, carrying the target's own
	// fresh MME-UE-S1AP-ID.
	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	req, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || req.ProcedureCode != s1ap.ProcHandoverResourceAllocation {
		t.Fatalf("expected HANDOVER REQUEST to target, got %T", lastPDU(t, target))
	}

	hoReq, err := s1ap.ParseHandoverRequest(req.Value)
	if err != nil {
		t.Fatalf("parse HANDOVER REQUEST: %v", err)
	}

	targetMME := hoReq.MMEUES1APID

	if targetMME == sourceMME || len(hoReq.ERABToBeSetup) != 1 ||
		hoReq.ERABToBeSetup[0].GTPTEID != 0x1111 || hoReq.SecurityContext.NextHopChainingCount != 2 {
		t.Fatalf("HANDOVER REQUEST = %+v (source-mme-id %d)", hoReq, sourceMME)
	}

	if s1ap.SecurityKey(wantNH) != hoReq.SecurityContext.NextHopParameter {
		t.Fatal("HANDOVER REQUEST carried the wrong Next Hop")
	}

	// The user plane must NOT be switched during preparation.
	if fsm := m.Session.(*fakeSessionManager); fsm.modifiedENB != (models.FTEID{}) {
		t.Fatalf("user plane switched during preparation: %+v", fsm.modifiedENB)
	}

	// HANDOVER REQUEST ACKNOWLEDGE (target id) → HANDOVER COMMAND to the source.
	const targetENBUEID s1ap.ENBUES1APID = 55

	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENBUEID,
		ERABAdmitted: []s1ap.ERABAdmittedItem{{
			ERABID:                s1ap.ERABID(mme.DefaultERABID),
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
		}},
		TargetToSource: s1ap.TransparentContainer{0xaa},
	}

	handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(target), successfulValue(t, mustMarshal(t, ack.Marshal)))

	cmd, ok := lastPDU(t, source).(*s1ap.SuccessfulOutcome)
	if !ok || cmd.ProcedureCode != s1ap.ProcHandoverPreparation {
		t.Fatalf("expected HANDOVER COMMAND to source, got %T", lastPDU(t, source))
	}

	if hoCmd, err := s1ap.ParseHandoverCommand(cmd.Value); err != nil || hoCmd.MMEUES1APID != sourceMME || hoCmd.ENBUES1APID != sourceENB {
		t.Fatalf("HANDOVER COMMAND addressed wrong source: %+v err %v", hoCmd, err)
	}

	// Still no user-plane switch before notify.
	if fsm := m.Session.(*fakeSessionManager); fsm.modifiedENB != (models.FTEID{}) {
		t.Fatalf("user plane switched before notify: %+v", fsm.modifiedENB)
	}

	// eNB STATUS TRANSFER (source id) → MME STATUS TRANSFER (target id) to the target.
	st := &s1ap.ENBStatusTransfer{MMEUES1APID: sourceMME, ENBUES1APID: sourceENB, Container: s1ap.StatusTransferContainer{0xde, 0xad}}
	handleENBStatusTransfer(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, st.Marshal)))

	mst, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || mst.ProcedureCode != s1ap.ProcMMEStatusTransfer {
		t.Fatalf("expected MME STATUS TRANSFER to target, got %T", lastPDU(t, target))
	}

	parsedMST, err := s1ap.ParseMMEStatusTransfer(mst.Value)
	if err != nil || parsedMST.MMEUES1APID != targetMME || parsedMST.ENBUES1APID != targetENBUEID {
		t.Fatalf("MME STATUS TRANSFER = %+v, err %v", parsedMST, err)
	}

	// HANDOVER NOTIFY (target id) → user-plane switch, association move, source release.
	notify := &s1ap.HandoverNotify{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENBUEID,
		EUTRANCGI:   s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		TAI:         s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
	}
	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), initiatingValue(t, mustMarshal(t, notify.Marshal)))

	wantFTEID := models.FTEID{TEID: 0x99, Addr: netip.AddrFrom4([4]byte{10, 4, 0, 2})}
	if fsm := m.Session.(*fakeSessionManager); fsm.modifiedENB != wantFTEID {
		t.Fatalf("ModifyEPSSession eNB F-TEID = %+v, want %+v", fsm.modifiedENB, wantFTEID)
	}

	// The UE's active connection is now the target connection (its own id).
	if ue.Conn().Conn() != target || ue.Conn().MMEUES1APID != targetMME || ue.Conn().ENBUES1APID != targetENBUEID || testPDN(ue).EnbFTEID != wantFTEID {
		t.Fatalf("association not moved to the target connection: conn=%v mme-id=%d enb-id=%d", ue.Conn().Conn() == target, ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID)
	}

	if ue.NCCForTest() != 2 || ue.NHForTest() != wantNH {
		t.Fatalf("key chain not committed: ncc=%d nh-match=%v", ue.NCCForTest(), ue.NHForTest() == wantNH)
	}

	if ue.HasHandoverForTest() {
		t.Fatal("handover context not cleared after notify")
	}

	// The source eNB received a UE Context Release Command addressed by the source's
	// own id.
	rel, ok := lastPDU(t, source).(*s1ap.InitiatingMessage)
	if !ok || rel.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command to source, got %T", lastPDU(t, source))
	}

	relCmd, err := s1ap.ParseUEContextReleaseCommand(rel.Value)
	if err != nil || relCmd.UES1APIDs.MMEUES1APID != sourceMME {
		t.Fatalf("source release addressed wrong id: %+v err %v", relCmd, err)
	}

	// A completed handover releases the source with "successful-handover" (value 2).
	if want := (s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 2}); relCmd.Cause != want {
		t.Fatalf("source release cause = %+v, want %+v (successful-handover)", relCmd.Cause, want)
	}

	// The source Release Complete (source id) removes the source connection without
	// disturbing the moved UE, which is found under the target id.
	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: sourceMME, ENBUES1APID: sourceENB}
	HandleUEContextReleaseComplete(m, context.Background(), mme.NewRadioForTest(source), successfulValue(t, mustMarshal(t, complete.Marshal)))

	if _, ok := m.LookupUe(sourceMME); ok {
		t.Fatal("source connection not removed by its Release Complete")
	}

	if got, ok := m.LookupUe(targetMME); !ok || got != ue {
		t.Fatal("UE not found under the target id after the source release")
	}

	if ue.Conn().Conn() != target {
		t.Fatal("UE association disturbed by the source Release Complete")
	}
}

// TestHandoverRequiredNoSecurityFails checks a UE without a security context is
// rejected with HANDOVER PREPARATION FAILURE and no HANDOVER REQUEST is sent.
func TestHandoverRequiredNoSecurityFails(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)
	ue.SetSecuredForTest(false)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

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

	if ue.HasHandoverForTest() {
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

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, req.Marshal)))

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

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if !ue.HasHandoverForTest() {
		t.Fatal("first handover did not start")
	}

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if !ue.HasHandoverForTest() {
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

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if !ue.HasHandoverForTest() {
		t.Fatal("handover did not start")
	}

	ncc, nh, conn := ue.NCCForTest(), ue.NHForTest(), ue.Conn().Conn()

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, samplePathSwitchRequest(ue)))

	if target.count() != 1 {
		t.Fatalf("expected one downlink (Path Switch Failure), got %d", target.count())
	}

	parsePathSwitchFailure(t, target.sent[0])

	if ue.NCCForTest() != ncc || ue.NHForTest() != nh || ue.Conn().Conn() != conn {
		t.Fatal("Path Switch advanced the key chain or moved the association during a handover")
	}
}

// TestHandoverRefusedWhileKeyChainBusy checks an S1 handover is refused while a
// Path Switch holds the key chain — the symmetric guard of the shared marker.
func TestHandoverRefusedWhileKeyChainBusy(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	ue.SetKeyChainBusyForTest(true) // a Path Switch is mid-advance

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if ue.HasHandoverForTest() {
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

// TestHandoverGuardSurvivesContextRelease checks that freeing a UE's connection
// mid-handover (e.g. a re-attach or detach) stops the handover guard timer, so its
// later expiry does not dereference the freed connection.
func TestHandoverGuardSurvivesContextRelease(t *testing.T) {
	m := newTestMME(t)
	ue, source, _ := handoverUE(t, m)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if !ue.HasHandoverForTest() {
		t.Fatal("handover did not start")
	}

	m.FreeUeConn(ue)

	if ue.Conn() != nil {
		t.Fatal("UE not idle after release")
	}

	// The orphaned supervision firing must not panic on the freed connection.
	m.FireHandoverGuardForTest(ue)
}

// TestHandoverSupervisionTimeoutAbandons checks the registry supervision is actually
// armed at PrepareHandover and fires at the TS1RELOCoverall deadline (TS 36.413 §8.4) —
// the end-to-end wiring, not just the abandonment action. The S1Handover procedure is
// polled via the registry (lock-synchronised against the timer goroutine, unlike the
// raw ue.handover field).
func TestHandoverSupervisionTimeoutAbandons(t *testing.T) {
	m := newTestMME(t)
	m.SetHandoverGuardTimeoutForTest(5 * time.Millisecond)
	ue, source, _ := handoverUE(t, m)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if !hasS1HandoverProc(ue) {
		t.Fatal("handover did not begin the S1Handover procedure")
	}

	deadline := time.Now().Add(2 * time.Second)
	for hasS1HandoverProc(ue) {
		if time.Now().After(deadline) {
			t.Fatal("supervision timeout did not abandon the handover")
		}

		time.Sleep(time.Millisecond)
	}
}

func hasS1HandoverProc(ue *mme.UeContext) bool {
	for _, p := range ue.ActiveProceduresForTest() {
		if p == "S1Handover" {
			return true
		}
	}

	return false
}

// TestHandoverFailureFailsToSource checks a HANDOVER FAILURE from the target ends
// the handover with a HANDOVER PREPARATION FAILURE to the source, the UE intact.
func TestHandoverFailureFailsToSource(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	fail := &s1ap.HandoverFailure{MMEUES1APID: targetMMEUEID(t, target), Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 12}}
	handleHandoverFailure(m, context.Background(), mme.NewRadioForTest(target), unsuccessfulValue(t, mustMarshal(t, fail.Marshal)))

	if ue.HasHandoverForTest() {
		t.Fatal("handover not cleared after failure")
	}

	uo, ok := lastPDU(t, source).(*s1ap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != s1ap.ProcHandoverPreparation {
		t.Fatalf("expected HANDOVER PREPARATION FAILURE to source, got %T", lastPDU(t, source))
	}

	// The source's preparation failure relays the target's HANDOVER FAILURE cause
	// (value 12), not a fixed one (TS 36.413 §8.4.1.3), mirroring the AMF.
	prepFail, err := s1ap.ParseHandoverPreparationFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse HANDOVER PREPARATION FAILURE: %v", err)
	}

	if want := (s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 12}); prepFail.Cause != want {
		t.Fatalf("preparation failure cause = %+v, want %+v (relayed target cause)", prepFail.Cause, want)
	}

	if ue.Conn().Conn() != source {
		t.Fatal("UE association moved on a failed handover")
	}
}

// TestHandoverCancelReleasesTarget checks a HANDOVER CANCEL after preparation
// releases the target context and acknowledges, leaving the UE on the source.
func TestHandoverCancelReleasesTarget(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, targetENBUEID := driveToPrepared(t, m, ue, source, target)

	cancel := &s1ap.HandoverCancel{MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: ue.Conn().ENBUES1APID, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5}}
	handleHandoverCancel(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, cancel.Marshal)))

	if ue.HasHandoverForTest() {
		t.Fatal("handover not cleared after cancel")
	}

	// The target received a UE Context Release Command.
	trel, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || trel.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command to target, got %T", lastPDU(t, target))
	}

	// The target release relays the source's HANDOVER CANCEL cause (value 5), not
	// "successful-handover" (TS 36.413 §8.4.5).
	relCmd, err := s1ap.ParseUEContextReleaseCommand(trel.Value)
	if err != nil {
		t.Fatalf("parse target release command: %v", err)
	}

	if want := (s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5}); relCmd.Cause != want {
		t.Fatalf("target release cause = %+v, want %+v (relayed cancel cause)", relCmd.Cause, want)
	}

	// The source received a HANDOVER CANCEL ACKNOWLEDGE.
	ack, ok := lastPDU(t, source).(*s1ap.SuccessfulOutcome)
	if !ok || ack.ProcedureCode != s1ap.ProcHandoverCancel {
		t.Fatalf("expected HANDOVER CANCEL ACKNOWLEDGE to source, got %T", lastPDU(t, source))
	}

	if ue.Conn().Conn() != source {
		t.Fatal("UE association moved on a cancelled handover")
	}

	// The target's Release Complete (target id) does not disturb the UE, which stays
	// on the source.
	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: targetMME, ENBUES1APID: targetENBUEID}
	HandleUEContextReleaseComplete(m, context.Background(), mme.NewRadioForTest(target), successfulValue(t, mustMarshal(t, complete.Marshal)))

	if _, ok := m.LookupUe(ue.Conn().MMEUES1APID); !ok {
		t.Fatal("UE removed by the target Release Complete")
	}
}

// TestHandoverCancelDuringPreparationReleasesTarget checks a HANDOVER CANCEL that
// arrives before the target acknowledges still releases the target's reserved
// resources (TS 36.413 §8.4.5), addressing it by MME-UE-S1AP-ID alone since its
// eNB-UE-S1AP-ID has not yet arrived. Without this the target context is orphaned.
func TestHandoverCancelDuringPreparationReleasesTarget(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	// Drive only to hoPreparing: HANDOVER REQUIRED processed and HANDOVER REQUEST sent
	// to the target, but no HANDOVER REQUEST ACKNOWLEDGE yet.
	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if target.count() != 1 {
		t.Fatalf("expected one HANDOVER REQUEST to the target, got %d", target.count())
	}

	cancel := &s1ap.HandoverCancel{MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: ue.Conn().ENBUES1APID, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5}}
	handleHandoverCancel(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, cancel.Marshal)))

	if ue.HasHandoverForTest() {
		t.Fatal("handover not cleared after cancel")
	}

	trel, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || trel.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command to the preparing target, got %T", lastPDU(t, target))
	}

	relCmd, err := s1ap.ParseUEContextReleaseCommand(trel.Value)
	if err != nil {
		t.Fatalf("parse target release command: %v", err)
	}

	// A still-preparing target has no known eNB-UE-S1AP-ID, so the release addresses it
	// by the MME-UE-S1AP-ID alone (the CHOICE's second alternative), not the pair.
	if relCmd.UES1APIDs.Pair {
		t.Error("a preparing target's release must use the MME-UE-S1AP-ID alone, not the pair")
	}

	if want := (s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5}); relCmd.Cause != want {
		t.Fatalf("target release cause = %+v, want %+v (relayed cancel cause)", relCmd.Cause, want)
	}

	ack, ok := lastPDU(t, source).(*s1ap.SuccessfulOutcome)
	if !ok || ack.ProcedureCode != s1ap.ProcHandoverCancel {
		t.Fatalf("expected HANDOVER CANCEL ACKNOWLEDGE to source, got %T", lastPDU(t, source))
	}

	if ue.Conn().Conn() != source {
		t.Fatal("UE association moved on a cancelled handover")
	}
}

// TestHandoverPartialAdmissionReleasesFailedPDN checks a multi-PDN UE whose target
// rejects one PDN's default bearer keeps the admitted PDN and releases the
// rejected one at notify (TS 23.401 §5.5.1.2.2 step 15).
func TestHandoverPartialAdmissionReleasesFailedPDN(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	// Second PDN connection on EBI 6.
	second := ue.EnsurePDN(6)
	second.Apn = "ims"
	second.Qci, second.Arp = 5, 7
	second.SgwFTEID = models.FTEID{TEID: 0x2222, Addr: netip.AddrFrom4([4]byte{10, 0, 0, 2})}

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	req, _ := s1ap.ParseHandoverRequest(lastPDU(t, target).(*s1ap.InitiatingMessage).Value)
	if len(req.ERABToBeSetup) != 2 {
		t.Fatalf("expected 2 E-RABs in HANDOVER REQUEST, got %d", len(req.ERABToBeSetup))
	}

	targetMME := req.MMEUES1APID

	const targetENBUEID s1ap.ENBUES1APID = 55

	// Target admits the default bearer (EBI 5), rejects EBI 6.
	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENBUEID,
		ERABAdmitted: []s1ap.ERABAdmittedItem{{
			ERABID:                s1ap.ERABID(mme.DefaultERABID),
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
		}},
		ERABFailedToSetup: []s1ap.ERABItem{{ERABID: 6, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}}},
		TargetToSource:    s1ap.TransparentContainer{0xaa},
	}
	handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(target), successfulValue(t, mustMarshal(t, ack.Marshal)))

	// HANDOVER COMMAND lists EBI 6 in the bearers-to-release list.
	cmd, _ := s1ap.ParseHandoverCommand(lastPDU(t, source).(*s1ap.SuccessfulOutcome).Value)
	if len(cmd.ERABToRelease) != 1 || cmd.ERABToRelease[0].ERABID != 6 {
		t.Fatalf("HANDOVER COMMAND release list = %+v", cmd.ERABToRelease)
	}

	notify := &s1ap.HandoverNotify{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENBUEID,
		EUTRANCGI:   s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		TAI:         s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
	}
	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), initiatingValue(t, mustMarshal(t, notify.Marshal)))

	// The rejected PDN was released and dropped; the admitted one survives.
	if fsm := m.Session.(*fakeSessionManager); !fsm.released {
		t.Fatal("rejected PDN session not released")
	}

	if m.LookupPDN(ue, 6) != nil {
		t.Fatal("rejected PDN connection not dropped")
	}

	if m.LookupPDN(ue, mme.DefaultERABID) == nil {
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
	ue.ForceHandoverCommittingForTest()

	targetBefore := target.count()

	cancel := &s1ap.HandoverCancel{MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: ue.Conn().ENBUES1APID, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5}}
	handleHandoverCancel(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, cancel.Marshal)))

	if !ue.HasHandoverForTest() {
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
	m.SetHandoverGuardTimeoutForTest(50 * time.Millisecond)

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

	cleared := !ue.HasHandoverForTest()
	conn := ue.Conn().Conn()

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

	// The abandoned target is released with "tS1relocoverall-expiry" (value 8), not
	// "successful-handover" (TS 36.413 §9.2.1.3).
	relCmd, err := s1ap.ParseUEContextReleaseCommand(rel.Value)
	if err != nil {
		t.Fatalf("parse target release command: %v", err)
	}

	if want := (s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 8}); relCmd.Cause != want {
		t.Fatalf("target release cause = %+v, want %+v (tS1relocoverall-expiry)", relCmd.Cause, want)
	}
}

// TestHandoverPartialAdmissionPromotesDefault checks that when the target rejects
// the UE's attach-default PDN but admits a secondary, the surviving PDN is promoted
// to the default so the UE retains a default PDN connection (TS 23.401 §5.5.1.2.2).
func TestHandoverPartialAdmissionPromotesDefault(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	second := ue.EnsurePDN(6)
	second.Apn = "ims"
	second.Qci, second.Arp = 5, 7
	second.SgwFTEID = models.FTEID{TEID: 0x2222, Addr: netip.AddrFrom4([4]byte{10, 0, 0, 2})}

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	targetMME := targetMMEUEID(t, target)

	const targetENBUEID s1ap.ENBUES1APID = 55

	// Target admits the secondary (EBI 6), rejects the attach default (EBI 5).
	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENBUEID,
		ERABAdmitted: []s1ap.ERABAdmittedItem{{
			ERABID:                6,
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
		}},
		ERABFailedToSetup: []s1ap.ERABItem{{ERABID: s1ap.ERABID(mme.DefaultERABID), Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}}},
		TargetToSource:    s1ap.TransparentContainer{0xaa},
	}
	handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(target), successfulValue(t, mustMarshal(t, ack.Marshal)))

	notify := &s1ap.HandoverNotify{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENBUEID,
		EUTRANCGI:   s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		TAI:         s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
	}
	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), initiatingValue(t, mustMarshal(t, notify.Marshal)))

	if m.LookupPDN(ue, mme.DefaultERABID) != nil {
		t.Fatal("rejected attach-default PDN not dropped")
	}

	defaultEBI := ue.DefaultEBI

	if defaultEBI != 6 {
		t.Fatalf("default EBI = %d, want 6 (promoted survivor)", defaultEBI)
	}
}

// TestHandoverTargetConnLossAborts checks that losing the target eNB association
// mid-handover aborts the handover and removes the target connection by its own
// id, leaving the UE on its source.
func TestHandoverTargetConnLossAborts(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, _ := driveToPrepared(t, m, ue, source, target)

	if _, ok := m.LookupUe(targetMME); !ok {
		t.Fatal("target connection not registered during preparation")
	}

	m.ReclaimUEsOnConnLossForTest(target)

	if ue.HasHandoverForTest() {
		t.Fatal("handover not aborted when the target association dropped")
	}

	if _, ok := m.LookupUe(targetMME); ok {
		t.Fatal("target connection not removed when its association dropped")
	}

	if ue.Conn() == nil || ue.Conn().Conn() != source {
		t.Fatal("UE not left on its source after the target association dropped")
	}
}

// TestHandoverSourceConnLossReclaims checks that losing the source eNB association
// mid-handover reclaims the UE to ECM-IDLE and drops the prepared target
// connection.
func TestHandoverSourceConnLossReclaims(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, _ := driveToPrepared(t, m, ue, source, target)

	m.ReclaimUEsOnConnLossForTest(source)

	if _, ok := m.LookupUe(targetMME); ok {
		t.Fatal("target connection not dropped when the source association dropped")
	}

	got, ok := m.LookupUeByIMSI(ue.IMSI())
	if !ok || got != ue || got.Connected() {
		t.Fatal("UE not reclaimed to ECM-IDLE on source association loss")
	}

	if got.HasHandoverForTest() {
		t.Fatal("handover not cleared on source association loss")
	}

	m.RemoveUe(ue) // stop the default-duration mobile reachable timer
}

// TestHandoverTargetResetAborts checks that an S1 Reset on the target eNB
// mid-handover aborts the handover (the UE stays on its source) without
// reclaiming the UE active on the source.
func TestHandoverTargetResetAborts(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, _ := driveToPrepared(t, m, ue, source, target)

	cause := s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: 0}
	handleReset(m, mme.NewRadioForTest(target), resetValue(t, &s1ap.Reset{Cause: cause, ResetType: s1ap.ResetType{All: true}}))

	if ue.HasHandoverForTest() {
		t.Fatal("handover not aborted by a reset on the target eNB")
	}

	if _, ok := m.LookupUe(targetMME); ok {
		t.Fatal("target connection not removed by the target reset")
	}

	if ue.Conn() == nil || ue.Conn().Conn() != source {
		t.Fatal("UE not left on its source after a reset on the target eNB")
	}
}

// TestHandoverSourceConnLossReleasesTarget checks that aborting a prepared handover
// by source-connection loss explicitly releases the target eNB, like the guard
// timer, without waiting for its own timeout.
func TestHandoverSourceConnLossReleasesTarget(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	driveToPrepared(t, m, ue, source, target)
	before := target.count()

	m.ReclaimUEsOnConnLossForTest(source)

	if target.count() != before+1 {
		t.Fatalf("target not released on source loss: count %d -> %d", before, target.count())
	}

	rel, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || rel.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command to target, got %T", lastPDU(t, target))
	}

	m.RemoveUe(ue) // stop the default-duration mobile reachable timer
}

// TestHandoverNotifyUEReleasedDuringSwitch checks the notify commit is guarded
// against a concurrent release during the unlocked user-plane switch: the UE is
// not resurrected onto the target.
func TestHandoverNotifyUEReleasedDuringSwitch(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

	base := m.Session.(*fakeSessionManager)
	m.Session = &hookSessionManager{fakeSessionManager: base, onModify: func() { m.FreeUeConn(ue) }}

	notify := &s1ap.HandoverNotify{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENB,
		EUTRANCGI:   s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		TAI:         s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
	}
	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), initiatingValue(t, mustMarshal(t, notify.Marshal)))

	if ue.Conn() != nil {
		t.Fatal("released UE resurrected onto the target by Handover Notify")
	}

	if ue.HasHandoverForTest() {
		t.Fatal("handover not cleared after the UE was released mid-switch")
	}
}

// TestHandoverRequestAcknowledge_NoMatchingPreparation_DoesNotReleaseLiveUE verifies
// that a duplicate/stale HANDOVER REQUEST ACKNOWLEDGE resolving to a UE with no
// matching handover preparation (e.g. one already handed over, whose association id
// is now its active one) is dropped — NOT answered with a UE Context Release, which
// would tear down a live UE. TS 36.413 §10.4 (response incompatible with receiver
// state) calls for local error handling.
func TestHandoverRequestAcknowledge_NoMatchingPreparation_DoesNotReleaseLiveUE(t *testing.T) {
	m := newTestMME(t)
	ue, source, _ := handoverUE(t, m) // registered, connected UE; no handover started

	before := len(source.sent)

	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID:    ue.Conn().MMEUES1APID,
		ENBUES1APID:    ue.Conn().ENBUES1APID,
		ERABAdmitted:   []s1ap.ERABAdmittedItem{{ERABID: s1ap.ERABID(mme.DefaultERABID), TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2}, GTPTEID: 0x99}},
		TargetToSource: s1ap.TransparentContainer{0xaa},
	}

	handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(source), successfulValue(t, mustMarshal(t, ack.Marshal)))

	if len(source.sent) != before {
		t.Fatalf("a stale acknowledge with no matching preparation must be dropped, but %d PDU(s) were sent (a UE Context Release would drop a live UE)", len(source.sent)-before)
	}
}
