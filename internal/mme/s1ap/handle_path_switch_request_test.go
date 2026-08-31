// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func pathSwitchValue(t *testing.T, req *s1ap.PathSwitchRequest) []byte {
	t.Helper()

	b, err := req.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	return pdu.(*s1ap.InitiatingMessage).Value
}

func parsePathSwitchAck(t *testing.T, pdu []byte) *s1ap.PathSwitchRequestAcknowledge {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	so, ok := msg.(*s1ap.SuccessfulOutcome)
	if !ok || so.ProcedureCode != s1ap.ProcPathSwitchRequest {
		t.Fatalf("expected Path Switch Request Acknowledge, got %T", msg)
	}

	ack, err := s1ap.ParsePathSwitchRequestAcknowledge(so.Value)
	if err != nil {
		t.Fatalf("parse ack: %v", err)
	}

	return ack
}

func parsePathSwitchFailure(t *testing.T, pdu []byte) *s1ap.PathSwitchRequestFailure {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	uo, ok := msg.(*s1ap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != s1ap.ProcPathSwitchRequest {
		t.Fatalf("expected Path Switch Request Failure, got %T", msg)
	}

	fail, err := s1ap.ParsePathSwitchRequestFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse failure: %v", err)
	}

	return fail
}

func pathSwitchUE(t *testing.T, m *mme.MME) *mme.UeContext {
	t.Helper()

	ue, _ := securedUE(t, m)
	testPDN(ue).Apn = "internet"
	ue.SetUESecurityCapability(eps.UENetworkCapability{EEA: 0xe0, EIA: 0xe0}, nil, mme.MintAuthProofForAttachRequest())
	ue.SetNCCForTest(1)

	var nh [32]byte
	for i := range nh {
		nh[i] = byte(0x40 + i)
	}

	ue.SetNHForTest(nh)

	return ue
}

func switchedDLItem() s1ap.ERABToBeSwitchedDLItem {
	return s1ap.ERABToBeSwitchedDLItem{
		ERABID:                s1ap.ERABID(mme.DefaultERABID),
		TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 2},
		GTPTEID:               0x99,
	}
}

func samplePathSwitchRequest(ue *mme.UeContext) *s1ap.PathSwitchRequest {
	return &s1ap.PathSwitchRequest{
		ENBUES1APID:            42,
		ERABToBeSwitchedDL:     []s1ap.ERABToBeSwitchedDLItem{switchedDLItem()},
		SourceMMEUES1APID:      ue.Conn().MMEUES1APID,
		EUTRANCGI:              s1ap.Ptr(s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1}),
		TAI:                    s1ap.Ptr(s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}),
		UESecurityCapabilities: s1ap.Ptr(s1ap.UESecurityCapabilities{EncryptionAlgorithms: 0xc000, IntegrityProtectionAlgorithms: 0xc000}),
	}
}

func TestPathSwitchSwitchesDownlinkAndAcks(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	wantNH, err := ue.DeriveNextNHForTest()
	if err != nil {
		t.Fatal(err)
	}

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, samplePathSwitchRequest(ue)))

	wantFTEID := models.FTEID{TEID: 0x99, Addr: netip.AddrFrom4([4]byte{10, 4, 0, 2})}
	if fsm := m.Session.(*fakeSessionManager); fsm.modifiedENB != wantFTEID {
		t.Fatalf("ModifyEPSSession eNB F-TEID = %+v, want %+v", fsm.modifiedENB, wantFTEID)
	}

	if ue.Conn().Conn() != target || ue.Conn().ENBUES1APID != 42 || testPDN(ue).EnbFTEID != wantFTEID {
		t.Fatalf("association not switched: conn=%v enb-id=%d fteid=%+v", ue.Conn().Conn() == target, ue.Conn().ENBUES1APID, testPDN(ue).EnbFTEID)
	}

	if ue.NCCForTest() != 2 || ue.NHForTest() != wantNH {
		t.Fatalf("key chain not advanced: ncc=%d nh-match=%v", ue.NCCForTest(), ue.NHForTest() == wantNH)
	}

	if target.count() != 1 {
		t.Fatalf("expected one downlink (Acknowledge), got %d", target.count())
	}

	ack := parsePathSwitchAck(t, target.sent[0])

	if ack.MMEUES1APID == nil || *ack.MMEUES1APID != ue.Conn().MMEUES1APID || ack.ENBUES1APID == nil || *ack.ENBUES1APID != 42 {
		t.Fatalf("ack UE IDs: mme=%#x enb=%d", ack.MMEUES1APID, ack.ENBUES1APID)
	}

	if ack.SecurityContext.NextHopChainingCount != 2 || s1ap.SecurityKey(wantNH) != ack.SecurityContext.NextHopParameter {
		t.Fatalf("ack security context = %+v, want ncc 2 / advanced NH", ack.SecurityContext)
	}

	if ack.UESecurityCapabilities != nil {
		t.Fatalf("ack replayed capabilities though they matched: %+v", ack.UESecurityCapabilities)
	}
}

func TestPathSwitchUnknownUEFails(t *testing.T) {
	m := newTestMME(t)

	req := &s1ap.PathSwitchRequest{
		ENBUES1APID:            42,
		ERABToBeSwitchedDL:     []s1ap.ERABToBeSwitchedDLItem{switchedDLItem()},
		SourceMMEUES1APID:      999,
		EUTRANCGI:              s1ap.Ptr(s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1}),
		TAI:                    s1ap.Ptr(s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}),
		UESecurityCapabilities: s1ap.Ptr(s1ap.UESecurityCapabilities{EncryptionAlgorithms: 0xc000, IntegrityProtectionAlgorithms: 0xc000}),
	}

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, req))

	if target.count() != 1 {
		t.Fatalf("expected one downlink (Failure), got %d", target.count())
	}

	if fail := parsePathSwitchFailure(t, target.sent[0]); fail.Cause == nil || *fail.Cause != causeUnknownMMEUES1APID {
		t.Fatalf("cause = %+v, want unknown-mme-ue-s1ap-id", fail.Cause)
	}
}

// TS 33.401 §7.2.8
func TestPathSwitchNoSecurityContextFails(t *testing.T) {
	m := newTestMME(t)
	ue := m.NewUe(&captureConn{}, 7)

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, samplePathSwitchRequest(ue)))

	if target.count() != 1 {
		t.Fatalf("expected one downlink (Failure), got %d", target.count())
	}

	if fail := parsePathSwitchFailure(t, target.sent[0]); fail.Cause == nil || *fail.Cause != causePathSwitchNoSecurity {
		t.Fatalf("cause = %+v, want authentication-failure", fail.Cause)
	}

	if m.Session.(*fakeSessionManager).modifiedENB != (models.FTEID{}) {
		t.Fatal("downlink switched despite missing security context")
	}
}

func TestPathSwitchDuplicateERABFails(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	req := samplePathSwitchRequest(ue)
	req.ERABToBeSwitchedDL = append(req.ERABToBeSwitchedDL, switchedDLItem())

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, req))

	if fail := parsePathSwitchFailure(t, target.sent[0]); fail.Cause == nil || *fail.Cause != causeMultipleERABInstances {
		t.Fatalf("cause = %+v, want multiple-E-RAB-ID-instances", fail.Cause)
	}

	if ue.NCCForTest() != 1 {
		t.Fatalf("key chain advanced on a rejected path switch: ncc=%d", ue.NCCForTest())
	}
}

func TestPathSwitchUnknownERABFails(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	req := samplePathSwitchRequest(ue)
	req.ERABToBeSwitchedDL[0].ERABID = s1ap.ERABID(mme.DefaultERABID + 1)

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, req))

	if fail := parsePathSwitchFailure(t, target.sent[0]); fail.Cause == nil || *fail.Cause != causePathSwitchUPFailure {
		t.Fatalf("cause = %+v, want transport-resource-unavailable", fail.Cause)
	}

	if m.Session.(*fakeSessionManager).modifiedENB != (models.FTEID{}) {
		t.Fatal("downlink switched for an unresolved E-RAB")
	}

	if ue.NCCForTest() != 1 || ue.Conn().Conn() == target {
		t.Fatalf("UE moved on a failed path switch: ncc=%d moved=%v", ue.NCCForTest(), ue.Conn().Conn() == target)
	}
}

func TestPathSwitchCapabilityMismatchReplaysStored(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	req := samplePathSwitchRequest(ue)
	req.UESecurityCapabilities = s1ap.Ptr(s1ap.UESecurityCapabilities{EncryptionAlgorithms: 0x8000, IntegrityProtectionAlgorithms: 0x8000})

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, req))

	ack := parsePathSwitchAck(t, target.sent[0])

	want := s1ap.UESecurityCapabilities{EncryptionAlgorithms: 0xc000, IntegrityProtectionAlgorithms: 0xc000}
	if ack.UESecurityCapabilities == nil || *ack.UESecurityCapabilities != want {
		t.Fatalf("replayed capabilities = %+v, want %+v", ack.UESecurityCapabilities, want)
	}
}

func TestPathSwitchUEReleasedDuringSwitch(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	base := m.Session.(*fakeSessionManager)
	m.Session = &hookSessionManager{fakeSessionManager: base, onModify: func() { m.FreeUeConn(ue) }}

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, samplePathSwitchRequest(ue)))

	if ue.Conn() != nil {
		t.Fatal("UE unexpectedly reconnected after being released mid-switch")
	}

	if target.count() != 1 {
		t.Fatalf("expected one downlink (Path Switch Failure), got %d", target.count())
	}

	parsePathSwitchFailure(t, target.sent[0])
}

// TS 36.413 §8.4.4.2
func TestPathSwitchPartialFailureReleasesUnswitchedERAB(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	req := samplePathSwitchRequest(ue)

	const unknownERAB = s1ap.ERABID(mme.DefaultERABID + 1)

	req.ERABToBeSwitchedDL = append(req.ERABToBeSwitchedDL, s1ap.ERABToBeSwitchedDLItem{
		ERABID:                unknownERAB,
		TransportLayerAddress: s1ap.TransportLayerAddress{10, 4, 0, 3},
		GTPTEID:               0x9a,
	})

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(target), pathSwitchValue(t, req))

	if target.count() != 1 {
		t.Fatalf("expected one downlink (Acknowledge), got %d", target.count())
	}

	ack := parsePathSwitchAck(t, target.sent[0])

	if len(ack.ERABToBeReleased) != 1 {
		t.Fatalf("expected 1 E-RAB in the To Be Released List, got %d", len(ack.ERABToBeReleased))
	}

	if ack.ERABToBeReleased[0].ERABID != unknownERAB {
		t.Fatalf("released E-RAB = %d, want %d", ack.ERABToBeReleased[0].ERABID, unknownERAB)
	}
}
