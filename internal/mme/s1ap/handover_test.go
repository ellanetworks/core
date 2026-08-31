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

var targetGlobalENBID = s1ap.GlobalENBID{
	PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10},
	ENBID:        s1ap.ENBID{Kind: s1ap.ENBIDMacro, Value: 2},
}

func handoverUE(t *testing.T, m *mme.MME) (*mme.UeContext, *captureConn, *captureConn) {
	t.Helper()

	ue, source := securedUE(t, m)
	p := testPDN(ue)
	p.Apn = "internet"
	p.Qci, p.Arp = 9, 8
	p.SgwFTEID = models.FTEID{TEID: 0x1111, Addr: netip.AddrFrom4([4]byte{10, 0, 0, 1})}

	ue.SetUESecurityCapability(eps.UENetworkCapability{EEA: 0xe0, EIA: 0xe0}, nil, mme.MintAuthProofForAttachRequest())

	ue.Ambr = &models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")}
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
		Cause:          s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 16}),
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

func driveToPrepared(t *testing.T, m *mme.MME, ue *mme.UeContext, source, target *captureConn) (s1ap.MMEUES1APID, s1ap.ENBUES1APID) {
	t.Helper()

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if target.count() != 1 {
		t.Fatalf("expected one HANDOVER REQUEST to the target, got %d", target.count())
	}

	targetMME := targetMMEUEID(t, target)

	const targetENBUEID s1ap.ENBUES1APID = 55

	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: s1ap.Ptr(targetMME),
		ENBUES1APID: s1ap.Ptr(targetENBUEID),
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

func TestHandoverHappyPath(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	sourceMME := ue.Conn().MMEUES1APID
	sourceENB := ue.Conn().ENBUES1APID

	wantNH, err := ue.DeriveNextNHForTest()
	if err != nil {
		t.Fatal(err)
	}

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

	if fsm := m.Session.(*fakeSessionManager); fsm.modifiedENB != (models.FTEID{}) {
		t.Fatalf("user plane switched during preparation: %+v", fsm.modifiedENB)
	}

	const targetENBUEID s1ap.ENBUES1APID = 55

	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: s1ap.Ptr(targetMME),
		ENBUES1APID: s1ap.Ptr(targetENBUEID),
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

	if fsm := m.Session.(*fakeSessionManager); fsm.modifiedENB != (models.FTEID{}) {
		t.Fatalf("user plane switched before notify: %+v", fsm.modifiedENB)
	}

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

	notify := &s1ap.HandoverNotify{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENBUEID,
		EUTRANCGI:   s1ap.Ptr(s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1}),
		TAI:         s1ap.Ptr(s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}),
	}
	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), initiatingValue(t, mustMarshal(t, notify.Marshal)))

	wantFTEID := models.FTEID{TEID: 0x99, Addr: netip.AddrFrom4([4]byte{10, 4, 0, 2})}
	if fsm := m.Session.(*fakeSessionManager); fsm.modifiedENB != wantFTEID {
		t.Fatalf("ModifyEPSSession eNB F-TEID = %+v, want %+v", fsm.modifiedENB, wantFTEID)
	}

	if ue.Conn().Conn() != target || ue.Conn().MMEUES1APID != targetMME || ue.Conn().ENBUES1APID != targetENBUEID || testPDN(ue).EnbFTEID != wantFTEID {
		t.Fatalf("association not moved to the target connection: conn=%v mme-id=%d enb-id=%d", ue.Conn().Conn() == target, ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID)
	}

	if ue.NCCForTest() != 2 || ue.NHForTest() != wantNH {
		t.Fatalf("key chain not committed: ncc=%d nh-match=%v", ue.NCCForTest(), ue.NHForTest() == wantNH)
	}

	if ue.HasHandoverForTest() {
		t.Fatal("handover context not cleared after notify")
	}

	rel, ok := lastPDU(t, source).(*s1ap.InitiatingMessage)
	if !ok || rel.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command to source, got %T", lastPDU(t, source))
	}

	relCmd, err := s1ap.ParseUEContextReleaseCommand(rel.Value)
	if err != nil || relCmd.UES1APIDs.MMEUES1APID != sourceMME {
		t.Fatalf("source release addressed wrong id: %+v err %v", relCmd, err)
	}

	if want := (s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 2}); relCmd.Cause == nil || *relCmd.Cause != want {
		t.Fatalf("source release cause = %+v, want %+v (successful-handover)", relCmd.Cause, want)
	}

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: s1ap.Ptr(sourceMME), ENBUES1APID: s1ap.Ptr(sourceENB)}
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
	if fail.Cause == nil || *fail.Cause != causeHandoverNoSecurity {
		t.Fatalf("cause = %+v, want authentication-failure", fail.Cause)
	}

	if ue.HasHandoverForTest() {
		t.Fatal("handover context left set on failure")
	}
}

func TestHandoverRequiredUnknownTargetFails(t *testing.T) {
	m := newTestMME(t)
	ue, source, _ := handoverUE(t, m)

	req := sampleHandoverRequired(ue)
	req.TargetID.TargeteNBID.GlobalENBID.ENBID.Value = 999

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, req.Marshal)))

	uo, ok := lastPDU(t, source).(*s1ap.UnsuccessfulOutcome)
	if !ok {
		t.Fatalf("expected HANDOVER PREPARATION FAILURE, got %T", lastPDU(t, source))
	}

	fail, _ := s1ap.ParseHandoverPreparationFailure(uo.Value)
	if fail.Cause == nil || *fail.Cause != causeUnknownTargetID {
		t.Fatalf("cause = %+v, want unknown-targetID", fail.Cause)
	}
}

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

// TS 33.401 §7.2.8
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

func TestHandoverRefusedWhileKeyChainBusy(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	ue.SetKeyChainBusyForTest(true)

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

	m.FireHandoverGuardForTest(ue)
}

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

func TestHandoverFailureFailsToSource(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	fail := &s1ap.HandoverFailure{MMEUES1APID: s1ap.Ptr(targetMMEUEID(t, target)), Cause: s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 12})}
	handleHandoverFailure(m, context.Background(), mme.NewRadioForTest(target), unsuccessfulValue(t, mustMarshal(t, fail.Marshal)))

	if ue.HasHandoverForTest() {
		t.Fatal("handover not cleared after failure")
	}

	uo, ok := lastPDU(t, source).(*s1ap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != s1ap.ProcHandoverPreparation {
		t.Fatalf("expected HANDOVER PREPARATION FAILURE to source, got %T", lastPDU(t, source))
	}

	prepFail, err := s1ap.ParseHandoverPreparationFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse HANDOVER PREPARATION FAILURE: %v", err)
	}

	if want := (s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 12}); prepFail.Cause == nil || *prepFail.Cause != want {
		t.Fatalf("preparation failure cause = %+v, want %+v (relayed target cause)", prepFail.Cause, want)
	}

	if ue.Conn().Conn() != source {
		t.Fatal("UE association moved on a failed handover")
	}
}

func TestHandoverCancelReleasesTarget(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, targetENBUEID := driveToPrepared(t, m, ue, source, target)

	cancel := &s1ap.HandoverCancel{MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: ue.Conn().ENBUES1APID, Cause: s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5})}
	handleHandoverCancel(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, cancel.Marshal)))

	if ue.HasHandoverForTest() {
		t.Fatal("handover not cleared after cancel")
	}

	trel, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || trel.ProcedureCode != s1ap.ProcUEContextRelease {
		t.Fatalf("expected UE Context Release Command to target, got %T", lastPDU(t, target))
	}

	relCmd, err := s1ap.ParseUEContextReleaseCommand(trel.Value)
	if err != nil {
		t.Fatalf("parse target release command: %v", err)
	}

	if want := (s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5}); relCmd.Cause == nil || *relCmd.Cause != want {
		t.Fatalf("target release cause = %+v, want %+v (relayed cancel cause)", relCmd.Cause, want)
	}

	ack, ok := lastPDU(t, source).(*s1ap.SuccessfulOutcome)
	if !ok || ack.ProcedureCode != s1ap.ProcHandoverCancel {
		t.Fatalf("expected HANDOVER CANCEL ACKNOWLEDGE to source, got %T", lastPDU(t, source))
	}

	if ue.Conn().Conn() != source {
		t.Fatal("UE association moved on a cancelled handover")
	}

	complete := &s1ap.UEContextReleaseComplete{MMEUES1APID: s1ap.Ptr(targetMME), ENBUES1APID: s1ap.Ptr(targetENBUEID)}
	HandleUEContextReleaseComplete(m, context.Background(), mme.NewRadioForTest(target), successfulValue(t, mustMarshal(t, complete.Marshal)))

	if _, ok := m.LookupUe(ue.Conn().MMEUES1APID); !ok {
		t.Fatal("UE removed by the target Release Complete")
	}
}

// TS 36.413 §8.4.5
func TestHandoverCancelDuringPreparationReleasesTarget(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if target.count() != 1 {
		t.Fatalf("expected one HANDOVER REQUEST to the target, got %d", target.count())
	}

	cancel := &s1ap.HandoverCancel{MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: ue.Conn().ENBUES1APID, Cause: s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5})}
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

	if relCmd.UES1APIDs.Pair {
		t.Error("a preparing target's release must use the MME-UE-S1AP-ID alone, not the pair")
	}

	if want := (s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5}); relCmd.Cause == nil || *relCmd.Cause != want {
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

// TS 23.401 §5.5.1.2.2
func TestHandoverPartialAdmissionReleasesFailedPDN(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

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

	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: s1ap.Ptr(targetMME),
		ENBUES1APID: s1ap.Ptr(targetENBUEID),
		ERABAdmitted: []s1ap.ERABAdmittedItem{{
			ERABID:                s1ap.ERABID(mme.DefaultERABID),
			TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
			GTPTEID:               0x99,
		}},
		ERABFailedToSetup: []s1ap.ERABItem{{ERABID: 6, Cause: s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}}},
		TargetToSource:    s1ap.TransparentContainer{0xaa},
	}
	handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(target), successfulValue(t, mustMarshal(t, ack.Marshal)))

	cmd, _ := s1ap.ParseHandoverCommand(lastPDU(t, source).(*s1ap.SuccessfulOutcome).Value)
	if len(cmd.ERABToRelease) != 1 || cmd.ERABToRelease[0].ERABID != 6 {
		t.Fatalf("HANDOVER COMMAND release list = %+v", cmd.ERABToRelease)
	}

	notify := &s1ap.HandoverNotify{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENBUEID,
		EUTRANCGI:   s1ap.Ptr(s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1}),
		TAI:         s1ap.Ptr(s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}),
	}
	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), initiatingValue(t, mustMarshal(t, notify.Marshal)))

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

func TestHandoverCancelDuringCommitIgnored(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	driveToPrepared(t, m, ue, source, target)

	ue.ForceHandoverCommittingForTest()

	targetBefore := target.count()

	cancel := &s1ap.HandoverCancel{MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: ue.Conn().ENBUES1APID, Cause: s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5})}
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

func TestHandoverGuardTimerAbandons(t *testing.T) {
	m := newTestMME(t)
	m.SetHandoverGuardTimeoutForTest(50 * time.Millisecond)

	ue, source, target := handoverUE(t, m)

	driveToPrepared(t, m, ue, source, target)

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

	relCmd, err := s1ap.ParseUEContextReleaseCommand(rel.Value)
	if err != nil {
		t.Fatalf("parse target release command: %v", err)
	}

	if want := (s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 8}); relCmd.Cause == nil || *relCmd.Cause != want {
		t.Fatalf("target release cause = %+v, want %+v (tS1relocoverall-expiry)", relCmd.Cause, want)
	}
}

// TS 23.401 §5.5.1.2.2
func TestHandoverPartialAdmissionKeepsSurvivingPDN(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	second := ue.EnsurePDN(6)
	second.Apn = "ims"
	second.Qci, second.Arp = 5, 7
	second.SgwFTEID = models.FTEID{TEID: 0x2222, Addr: netip.AddrFrom4([4]byte{10, 0, 0, 2})}

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source), initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	targetMME := targetMMEUEID(t, target)

	const targetENBUEID s1ap.ENBUES1APID = 55

	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID: s1ap.Ptr(targetMME),
		ENBUES1APID: s1ap.Ptr(targetENBUEID),
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
		EUTRANCGI:   s1ap.Ptr(s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1}),
		TAI:         s1ap.Ptr(s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}),
	}
	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), initiatingValue(t, mustMarshal(t, notify.Marshal)))

	if m.LookupPDN(ue, mme.DefaultERABID) != nil {
		t.Fatal("rejected attach-default PDN not dropped")
	}

	survivor := m.LookupPDN(ue, 6)
	if survivor == nil {
		t.Fatal("the admitted PDN connection must survive the handover")
	}

	if survivor.EnbFTEID.TEID != 0x99 {
		t.Errorf("survivor downlink not switched to the target: %+v", survivor.EnbFTEID)
	}

	if ue.BearerReleaseOnly(survivor) {
		t.Error("releasing the last remaining PDN connection must not be treated as bearer-only")
	}
}

func handoverNotify(targetMME s1ap.MMEUES1APID, targetENB s1ap.ENBUES1APID) *s1ap.HandoverNotify {
	return &s1ap.HandoverNotify{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENB,
		EUTRANCGI:   s1ap.Ptr(s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1}),
		TAI:         s1ap.Ptr(s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}),
	}
}

// TS 36.413 §10.6
func TestHandoverNotifyUnknownMMEUES1APIDSendsErrorIndication(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	_, targetENB := driveToPrepared(t, m, ue, source, target)

	const unknownMME s1ap.MMEUES1APID = 0xFFFF00
	if _, ok := m.LookupUe(unknownMME); ok {
		t.Fatalf("test precondition: MME-UE-S1AP-ID %d must be unallocated", unknownMME)
	}

	before := target.count()

	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), initiatingValue(t, mustMarshal(t, handoverNotify(unknownMME, targetENB).Marshal)))

	if target.count() != before+1 {
		t.Fatalf("expected one Error Indication to the target, got %d PDU(s)", target.count()-before)
	}

	ind := parseOutboundErrorIndication(t, target.sent[before])

	if ind.MMEUES1APID == nil || *ind.MMEUES1APID != unknownMME {
		t.Fatalf("Error Indication MME-UE-S1AP-ID = %v, want %d", ind.MMEUES1APID, unknownMME)
	}

	if ind.ENBUES1APID == nil || *ind.ENBUES1APID != targetENB {
		t.Fatalf("Error Indication eNB-UE-S1AP-ID = %v, want %d", ind.ENBUES1APID, targetENB)
	}

	if ind.Cause == nil || *ind.Cause != causeUnknownMMEUES1APID {
		t.Fatalf("Error Indication cause = %v, want unknown-mme-ue-s1ap-id", ind.Cause)
	}

	if !ue.HasHandoverForTest() {
		t.Fatal("in-flight handover torn down by a Handover Notify with an unknown MME-UE-S1AP-ID")
	}
}

// TS 36.413 §10.6
func TestHandoverNotifyInconsistentENBUES1APIDSendsErrorIndication(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

	const wrongENB s1ap.ENBUES1APID = 77
	if wrongENB == targetENB {
		t.Fatal("test precondition: wrongENB must differ from the prepared target eNB-UE-S1AP-ID")
	}

	before := target.count()

	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), initiatingValue(t, mustMarshal(t, handoverNotify(targetMME, wrongENB).Marshal)))

	if target.count() != before+1 {
		t.Fatalf("expected one Error Indication to the target, got %d PDU(s)", target.count()-before)
	}

	ind := parseOutboundErrorIndication(t, target.sent[before])

	if ind.MMEUES1APID == nil || *ind.MMEUES1APID != targetMME {
		t.Fatalf("Error Indication MME-UE-S1AP-ID = %v, want %d", ind.MMEUES1APID, targetMME)
	}

	if ind.ENBUES1APID == nil || *ind.ENBUES1APID != wrongENB {
		t.Fatalf("Error Indication eNB-UE-S1AP-ID = %v, want %d", ind.ENBUES1APID, wrongENB)
	}

	if ind.Cause == nil || ind.Cause.Group != s1ap.CauseGroupRadioNetwork {
		t.Fatalf("Error Indication cause = %v, want a radio-network cause", ind.Cause)
	}

	if fsm := m.Session.(*fakeSessionManager); fsm.modifiedENB != (models.FTEID{}) {
		t.Fatalf("user plane switched for an inconsistent Handover Notify: %+v", fsm.modifiedENB)
	}

	if !ue.HasHandoverForTest() {
		t.Fatal("in-flight handover committed by a Handover Notify with an inconsistent eNB-UE-S1AP-ID")
	}
}

// TS 36.413 §10.6
func TestHandoverNotifyStaleDuplicateAfterCompletion(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

	notify := initiatingValue(t, mustMarshal(t, handoverNotify(targetMME, targetENB).Marshal))
	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), notify)

	if ue.Conn() == nil || ue.Conn().MMEUES1APID != targetMME || ue.Conn().ENBUES1APID != targetENB {
		t.Fatal("handover did not complete onto the target")
	}

	before := target.count()

	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), notify)

	if target.count() != before {
		t.Fatalf("stale Handover Notify drew %d response PDU(s); expected none", target.count()-before)
	}

	if ue.Conn() == nil || ue.Conn().MMEUES1APID != targetMME || ue.Conn().ENBUES1APID != targetENB {
		t.Fatal("live UE torn down by a stale Handover Notify")
	}
}

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

	m.RemoveUe(ue)
}

func TestHandoverTargetResetAborts(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, _ := driveToPrepared(t, m, ue, source, target)

	cause := s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: 0}
	handleReset(m, context.Background(), mme.NewRadioForTest(target), resetValue(t, &s1ap.Reset{Cause: s1ap.Ptr(cause), ResetType: s1ap.ResetType{All: true}}))

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

	m.RemoveUe(ue)
}

func TestHandoverNotifyUEReleasedDuringSwitch(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, targetENB := driveToPrepared(t, m, ue, source, target)

	base := m.Session.(*fakeSessionManager)
	m.Session = &hookSessionManager{fakeSessionManager: base, onModify: func() { m.FreeUeConn(ue) }}

	notify := &s1ap.HandoverNotify{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENB,
		EUTRANCGI:   s1ap.Ptr(s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1}),
		TAI:         s1ap.Ptr(s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}),
	}
	handleHandoverNotify(m, context.Background(), mme.NewRadioForTest(target), initiatingValue(t, mustMarshal(t, notify.Marshal)))

	if ue.Conn() != nil {
		t.Fatal("released UE resurrected onto the target by Handover Notify")
	}

	if ue.HasHandoverForTest() {
		t.Fatal("handover not cleared after the UE was released mid-switch")
	}
}

// TS 36.413 §10.4
func TestHandoverRequestAcknowledge_NoMatchingPreparation_DoesNotReleaseLiveUE(t *testing.T) {
	m := newTestMME(t)
	ue, source, _ := handoverUE(t, m)

	before := len(source.sent)

	ack := &s1ap.HandoverRequestAcknowledge{
		MMEUES1APID:    s1ap.Ptr(ue.Conn().MMEUES1APID),
		ENBUES1APID:    s1ap.Ptr(ue.Conn().ENBUES1APID),
		ERABAdmitted:   []s1ap.ERABAdmittedItem{{ERABID: s1ap.ERABID(mme.DefaultERABID), TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2}, GTPTEID: 0x99}},
		TargetToSource: s1ap.TransparentContainer{0xaa},
	}

	handleHandoverRequestAcknowledge(m, context.Background(), mme.NewRadioForTest(source), successfulValue(t, mustMarshal(t, ack.Marshal)))

	if len(source.sent) != before {
		t.Fatalf("a stale acknowledge with no matching preparation must be dropped, but %d PDU(s) were sent (a UE Context Release would drop a live UE)", len(source.sent)-before)
	}
}

func TestHandoverNHAdvancedAtPreparation(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	nh0, ncc0 := ue.NHForTest(), ue.NCCForTest()

	handleHandoverRequired(m, context.Background(), mme.NewRadioForTest(source),
		initiatingValue(t, mustMarshal(t, sampleHandoverRequired(ue).Marshal)))

	if target.count() != 1 {
		t.Fatalf("expected a HANDOVER REQUEST to the target, got %d messages", target.count())
	}

	afterPrepare, afterPrepareNCC := ue.NHForTest(), ue.NCCForTest()

	if afterPrepare == nh0 {
		t.Fatal("preparation must advance the live NH")
	}

	if afterPrepareNCC != (ncc0+1)&0x07 {
		t.Fatalf("NCC after preparation = %d, want %d", afterPrepareNCC, (ncc0+1)&0x07)
	}

	m.FireHandoverGuardForTest(ue)

	if ue.NHForTest() != afterPrepare || ue.NCCForTest() != afterPrepareNCC {
		t.Error("an abandoned handover must not roll the key chain back")
	}
}

// TS 36.413 §8.4.5.1
func TestHandoverCancelFromTheTargetLeavesTheHandoverStanding(t *testing.T) {
	m := newTestMME(t)
	ue, source, target := handoverUE(t, m)

	targetMME, targetENBUEID := driveToPrepared(t, m, ue, source, target)

	cancel := &s1ap.HandoverCancel{
		MMEUES1APID: targetMME,
		ENBUES1APID: targetENBUEID,
		Cause:       s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 5}),
	}
	handleHandoverCancel(m, context.Background(), mme.NewRadioForTest(target), initiatingValue(t, mustMarshal(t, cancel.Marshal)))

	if !ue.HasHandoverForTest() {
		t.Fatal("the target cancelled the source's handover")
	}

	answer, ok := lastPDU(t, target).(*s1ap.InitiatingMessage)
	if !ok || answer.ProcedureCode != s1ap.ProcErrorIndication {
		t.Fatalf("last message to the target is %T, want an Error Indication", lastPDU(t, target))
	}

	if lastPDU(t, source) == nil {
		t.Fatal("no message on the source")
	}

	if ack, isAck := lastPDU(t, source).(*s1ap.SuccessfulOutcome); isAck && ack.ProcedureCode == s1ap.ProcHandoverCancel {
		t.Error("the source was told its handover had been cancelled")
	}
}
