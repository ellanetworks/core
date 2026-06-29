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

// pathSwitchValue marshals a PATH SWITCH REQUEST and returns the initiatingMessage
// open-type payload the handler consumes.
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

// pathSwitchUE returns a secured UE seeded as if Initial Context Setup had run:
// the X2 key chain is at NCC=1 with a known NH, and the UE network capability is
// set so the replayed-capability comparison runs against real values.
func pathSwitchUE(t *testing.T, m *mme.MME) *mme.UeContext {
	t.Helper()

	ue, _ := securedUE(t, m)
	testPDN(ue).Apn = "internet"
	ue.UeNetCap = eps.UENetworkCapability{EEA: 0xe0, EIA: 0xe0}.Marshal()
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
		ENBUES1APID:        42,
		ERABToBeSwitchedDL: []s1ap.ERABToBeSwitchedDLItem{switchedDLItem()},
		SourceMMEUES1APID:  ue.S1.MMEUES1APID,
		EUTRANCGI:          s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		TAI:                s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
		// Matches the stored capabilities (EEA/EIA 0xe0, EEA0/EIA0 bit dropped).
		UESecurityCapabilities: s1ap.UESecurityCapabilities{EncryptionAlgorithms: 0xc000, IntegrityProtectionAlgorithms: 0xc000},
	}
}

// TestPathSwitchSwitchesDownlinkAndAcks drives the happy path: the downlink is
// switched to the target eNB, the S1 association moves, the {NH, NCC} chain
// advances, and a Path Switch Request Acknowledge is sent.
func TestPathSwitchSwitchesDownlinkAndAcks(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	wantNH, err := ue.DeriveNextNHForTest()
	if err != nil {
		t.Fatal(err)
	}

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), target, pathSwitchValue(t, samplePathSwitchRequest(ue)))

	// Downlink switched to the new eNB S1-U endpoint.
	wantFTEID := models.FTEID{TEID: 0x99, Addr: netip.AddrFrom4([4]byte{10, 4, 0, 2})}
	if fsm := m.Session.(*fakeSessionManager); fsm.modifiedENB != wantFTEID {
		t.Fatalf("ModifyEPSSession eNB F-TEID = %+v, want %+v", fsm.modifiedENB, wantFTEID)
	}

	// S1 association moved to the target eNB.
	if ue.S1.Conn() != target || ue.S1.ENBUES1APID != 42 || testPDN(ue).EnbFTEID != wantFTEID {
		t.Fatalf("association not switched: conn=%v enb-id=%d fteid=%+v", ue.S1.Conn() == target, ue.S1.ENBUES1APID, testPDN(ue).EnbFTEID)
	}

	// Key chain advanced: NCC 1 -> 2 and NH = KDF(KASME, previous NH).
	if ue.NCCForTest() != 2 || ue.NHForTest() != wantNH {
		t.Fatalf("key chain not advanced: ncc=%d nh-match=%v", ue.NCCForTest(), ue.NHForTest() == wantNH)
	}

	if target.count() != 1 {
		t.Fatalf("expected one downlink (Acknowledge), got %d", target.count())
	}

	ack := parsePathSwitchAck(t, target.sent[0])

	if ack.MMEUES1APID != ue.S1.MMEUES1APID || ack.ENBUES1APID != 42 {
		t.Fatalf("ack UE IDs: mme=%#x enb=%d", ack.MMEUES1APID, ack.ENBUES1APID)
	}

	if ack.SecurityContext.NextHopChainingCount != 2 || s1ap.SecurityKey(wantNH) != ack.SecurityContext.NextHopParameter {
		t.Fatalf("ack security context = %+v, want ncc 2 / advanced NH", ack.SecurityContext)
	}

	if ack.UESecurityCapabilities != nil {
		t.Fatalf("ack replayed capabilities though they matched: %+v", ack.UESecurityCapabilities)
	}
}

// TestPathSwitchUnknownUEFails checks an unresolvable Source MME UE S1AP ID is
// rejected with cause unknown-mme-ue-s1ap-id and no UE state is touched.
func TestPathSwitchUnknownUEFails(t *testing.T) {
	m := newTestMME(t)

	req := &s1ap.PathSwitchRequest{
		ENBUES1APID:            42,
		ERABToBeSwitchedDL:     []s1ap.ERABToBeSwitchedDLItem{switchedDLItem()},
		SourceMMEUES1APID:      999,
		EUTRANCGI:              s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		TAI:                    s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
		UESecurityCapabilities: s1ap.UESecurityCapabilities{EncryptionAlgorithms: 0xc000, IntegrityProtectionAlgorithms: 0xc000},
	}

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), target, pathSwitchValue(t, req))

	if target.count() != 1 {
		t.Fatalf("expected one downlink (Failure), got %d", target.count())
	}

	if fail := parsePathSwitchFailure(t, target.sent[0]); fail.Cause != causeUnknownMMEUES1APID {
		t.Fatalf("cause = %+v, want unknown-mme-ue-s1ap-id", fail.Cause)
	}
}

// TestPathSwitchNoSecurityContextFails checks a UE without a security context is
// rejected with cause authentication-failure (TS 33.401 §7.2.8).
func TestPathSwitchNoSecurityContextFails(t *testing.T) {
	m := newTestMME(t)
	ue := m.NewUe(&captureConn{}, 7) // not secured

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), target, pathSwitchValue(t, samplePathSwitchRequest(ue)))

	if target.count() != 1 {
		t.Fatalf("expected one downlink (Failure), got %d", target.count())
	}

	if fail := parsePathSwitchFailure(t, target.sent[0]); fail.Cause != causePathSwitchNoSecurity {
		t.Fatalf("cause = %+v, want authentication-failure", fail.Cause)
	}

	// The downlink must not have been switched.
	if m.Session.(*fakeSessionManager).modifiedENB != (models.FTEID{}) {
		t.Fatal("downlink switched despite missing security context")
	}
}

// TestPathSwitchDuplicateERABFails checks a to-be-switched list repeating an
// E-RAB ID is rejected with cause multiple-E-RAB-ID-instances (TS 36.413
// §8.4.4.4).
func TestPathSwitchDuplicateERABFails(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	req := samplePathSwitchRequest(ue)
	req.ERABToBeSwitchedDL = append(req.ERABToBeSwitchedDL, switchedDLItem())

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), target, pathSwitchValue(t, req))

	if fail := parsePathSwitchFailure(t, target.sent[0]); fail.Cause != causeMultipleERABInstances {
		t.Fatalf("cause = %+v, want multiple-E-RAB-ID-instances", fail.Cause)
	}

	if ue.NCCForTest() != 1 {
		t.Fatalf("key chain advanced on a rejected path switch: ncc=%d", ue.NCCForTest())
	}
}

// TestPathSwitchUnknownERABFails checks that a request whose only E-RAB does not
// resolve to a PDN connection switches nothing and is rejected (TS 36.413
// §8.4.4.3), leaving the UE on the source eNB.
func TestPathSwitchUnknownERABFails(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	req := samplePathSwitchRequest(ue)
	req.ERABToBeSwitchedDL[0].ERABID = s1ap.ERABID(mme.DefaultERABID + 1) // not the default bearer

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), target, pathSwitchValue(t, req))

	if fail := parsePathSwitchFailure(t, target.sent[0]); fail.Cause != causePathSwitchUPFailure {
		t.Fatalf("cause = %+v, want transport-resource-unavailable", fail.Cause)
	}

	if m.Session.(*fakeSessionManager).modifiedENB != (models.FTEID{}) {
		t.Fatal("downlink switched for an unresolved E-RAB")
	}

	if ue.NCCForTest() != 1 || ue.S1.Conn() == target {
		t.Fatalf("UE moved on a failed path switch: ncc=%d moved=%v", ue.NCCForTest(), ue.S1.Conn() == target)
	}
}

// TestPathSwitchCapabilityMismatchReplaysStored checks that when the target eNB
// reports UE security capabilities differing from the stored ones, the MME
// replays its stored values in the Acknowledge and does not overwrite them.
func TestPathSwitchCapabilityMismatchReplaysStored(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	req := samplePathSwitchRequest(ue)
	req.UESecurityCapabilities = s1ap.UESecurityCapabilities{EncryptionAlgorithms: 0x8000, IntegrityProtectionAlgorithms: 0x8000}

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), target, pathSwitchValue(t, req))

	ack := parsePathSwitchAck(t, target.sent[0])

	want := s1ap.UESecurityCapabilities{EncryptionAlgorithms: 0xc000, IntegrityProtectionAlgorithms: 0xc000}
	if ack.UESecurityCapabilities == nil || *ack.UESecurityCapabilities != want {
		t.Fatalf("replayed capabilities = %+v, want %+v", ack.UESecurityCapabilities, want)
	}
}

// TestPathSwitchUEReleasedDuringSwitch checks the commit is guarded against a
// concurrent release that frees ue.s1 during the unlocked user-plane switch: the
// path switch fails gracefully rather than dereferencing a nil connection.
func TestPathSwitchUEReleasedDuringSwitch(t *testing.T) {
	m := newTestMME(t)
	ue := pathSwitchUE(t, m)

	base := m.Session.(*fakeSessionManager)
	m.Session = &hookSessionManager{fakeSessionManager: base, onModify: func() { m.FreeS1Conn(ue) }}

	target := &captureConn{}
	handlePathSwitchRequest(m, context.Background(), target, pathSwitchValue(t, samplePathSwitchRequest(ue)))

	if ue.S1 != nil {
		t.Fatal("UE unexpectedly reconnected after being released mid-switch")
	}

	if target.count() != 1 {
		t.Fatalf("expected one downlink (Path Switch Failure), got %d", target.count())
	}

	parsePathSwitchFailure(t, target.sent[0])
}
