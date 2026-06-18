// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"
)

func sampledPathSwitchRequest() *PathSwitchRequest {
	return &PathSwitchRequest{
		ENBUES1APID: 2,
		ERABToBeSwitchedDL: []ERABToBeSwitchedDLItem{
			{ERABID: 5, TransportLayerAddress: TransportLayerAddress{10, 1, 2, 3}, GTPTEID: 0x01020304},
		},
		SourceMMEUES1APID:      0x020000bf,
		EUTRANCGI:              EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x123c601},
		TAI:                    TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
		UESecurityCapabilities: UESecurityCapabilities{EncryptionAlgorithms: 0xc000, IntegrityProtectionAlgorithms: 0xc000},
	}
}

// TestPathSwitchRequestRoundTrip encodes a PATH SWITCH REQUEST, decodes it via the
// generic Unmarshal envelope, and checks every modeled field survives.
func TestPathSwitchRequestRoundTrip(t *testing.T) {
	in := sampledPathSwitchRequest()

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcPathSwitchRequest {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParsePathSwitchRequest(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.ENBUES1APID != in.ENBUES1APID || out.SourceMMEUES1APID != in.SourceMMEUES1APID {
		t.Fatalf("UE IDs: enb=%d srcmme=%#x", out.ENBUES1APID, out.SourceMMEUES1APID)
	}

	if out.EUTRANCGI != in.EUTRANCGI || out.TAI != in.TAI {
		t.Fatalf("location: cgi=%+v tai=%+v", out.EUTRANCGI, out.TAI)
	}

	if out.UESecurityCapabilities != in.UESecurityCapabilities {
		t.Fatalf("security capabilities = %+v, want %+v", out.UESecurityCapabilities, in.UESecurityCapabilities)
	}

	if len(out.ERABToBeSwitchedDL) != 1 {
		t.Fatalf("E-RAB list len = %d, want 1", len(out.ERABToBeSwitchedDL))
	}

	got := out.ERABToBeSwitchedDL[0]
	want := in.ERABToBeSwitchedDL[0]

	if got.ERABID != want.ERABID || got.GTPTEID != want.GTPTEID ||
		!bytes.Equal(got.TransportLayerAddress, want.TransportLayerAddress) {
		t.Fatalf("E-RAB item = %+v, want %+v", got, want)
	}
}

// TestPathSwitchRequestEmptyERABListRejected checks the mandatory E-RAB
// to-be-switched list is SIZE(1..256): an empty list cannot be encoded.
func TestPathSwitchRequestEmptyERABListRejected(t *testing.T) {
	req := sampledPathSwitchRequest()
	req.ERABToBeSwitchedDL = nil

	if _, err := req.Marshal(); err == nil {
		t.Fatal("expected Marshal to fail with an empty E-RAB to-be-switched list")
	}
}

// TestPathSwitchRequestAcknowledgeRoundTrip covers the ACK with the mandatory
// Security Context plus the optional UE-AMBR and replayed UE security capabilities.
func TestPathSwitchRequestAcknowledgeRoundTrip(t *testing.T) {
	var nh SecurityKey
	for i := range nh {
		nh[i] = byte(i + 1)
	}

	in := &PathSwitchRequestAcknowledge{
		MMEUES1APID:               0x020000bf,
		ENBUES1APID:               2,
		UEAggregateMaximumBitRate: &UEAggregateMaximumBitRate{DL: 1_000_000, UL: 500_000},
		SecurityContext:           SecurityContext{NextHopChainingCount: 3, NextHopParameter: nh},
		UESecurityCapabilities:    &UESecurityCapabilities{EncryptionAlgorithms: 0x8000, IntegrityProtectionAlgorithms: 0x8000},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcPathSwitchRequest {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParsePathSwitchRequestAcknowledge(so.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID {
		t.Fatalf("UE IDs: mme=%#x enb=%d", out.MMEUES1APID, out.ENBUES1APID)
	}

	if out.SecurityContext != in.SecurityContext {
		t.Fatalf("security context = %+v, want %+v", out.SecurityContext, in.SecurityContext)
	}

	if out.UEAggregateMaximumBitRate == nil || *out.UEAggregateMaximumBitRate != *in.UEAggregateMaximumBitRate {
		t.Fatalf("UE-AMBR = %+v, want %+v", out.UEAggregateMaximumBitRate, in.UEAggregateMaximumBitRate)
	}

	if out.UESecurityCapabilities == nil || *out.UESecurityCapabilities != *in.UESecurityCapabilities {
		t.Fatalf("UE security capabilities = %+v, want %+v", out.UESecurityCapabilities, in.UESecurityCapabilities)
	}
}

// TestPathSwitchRequestAcknowledgeMinimal covers the ACK with only the mandatory
// IEs (no UE-AMBR, no replayed capabilities).
func TestPathSwitchRequestAcknowledgeMinimal(t *testing.T) {
	in := &PathSwitchRequestAcknowledge{
		MMEUES1APID:     7,
		ENBUES1APID:     2,
		SecurityContext: SecurityContext{NextHopChainingCount: 1},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParsePathSwitchRequestAcknowledge(pdu.(*SuccessfulOutcome).Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.UEAggregateMaximumBitRate != nil || out.UESecurityCapabilities != nil {
		t.Fatalf("unexpected optional IEs: ambr=%v caps=%v", out.UEAggregateMaximumBitRate, out.UESecurityCapabilities)
	}

	if out.SecurityContext.NextHopChainingCount != 1 {
		t.Fatalf("NCC = %d, want 1", out.SecurityContext.NextHopChainingCount)
	}
}

// TestPathSwitchRequestFailureRoundTrip covers the FAILURE message.
func TestPathSwitchRequestFailureRoundTrip(t *testing.T) {
	in := &PathSwitchRequestFailure{
		MMEUES1APID: 7,
		ENBUES1APID: 2,
		Cause:       Cause{Group: CauseGroupRadioNetwork, Value: 4},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	uo, ok := pdu.(*UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ProcPathSwitchRequest {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParsePathSwitchRequestFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID || out.Cause != in.Cause {
		t.Fatalf("failure = %+v, want %+v", out, in)
	}
}
