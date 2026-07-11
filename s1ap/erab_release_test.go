// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"
)

func TestERABReleaseCommandRoundTrips(t *testing.T) {
	ambr := UEAggregateMaximumBitRate{DL: 1000000000, UL: 500000000}

	in := &ERABReleaseCommand{
		MMEUES1APID:               42,
		ENBUES1APID:               1,
		UEAggregateMaximumBitRate: &ambr,
		ERABToBeReleased: []ERABItem{
			{ERABID: 6, Cause: Cause{Group: CauseGroupNAS, Value: 2}}, // normal-release
		},
		NASPDU: NASPDU{0x02, 0x06, 0xcd}, // a Deactivate EPS Bearer Context Request, abbreviated
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcERABRelease {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseERABReleaseCommand(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.MMEUES1APID != 42 || out.ENBUES1APID != 1 || len(out.ERABToBeReleased) != 1 {
		t.Fatalf("header mismatch: %+v", out)
	}

	if out.UEAggregateMaximumBitRate == nil || *out.UEAggregateMaximumBitRate != ambr {
		t.Fatalf("UE-AMBR mismatch: %+v", out.UEAggregateMaximumBitRate)
	}

	if out.ERABToBeReleased[0].ERABID != 6 || out.ERABToBeReleased[0].Cause != in.ERABToBeReleased[0].Cause {
		t.Fatalf("E-RAB item mismatch: %+v", out.ERABToBeReleased[0])
	}

	if !bytes.Equal(out.NASPDU, in.NASPDU) {
		t.Fatalf("NAS-PDU = %x, want %x", out.NASPDU, in.NASPDU)
	}
}

func TestERABReleaseResponseRoundTrips(t *testing.T) {
	in := &ERABReleaseResponse{
		MMEUES1APID:  42,
		ENBUES1APID:  1,
		ERABReleased: []ERABReleaseItemBearerRelComp{{ERABID: 6}},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcERABRelease {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseERABReleaseResponse(so.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.MMEUES1APID != 42 || out.ENBUES1APID != 1 || len(out.ERABReleased) != 1 || out.ERABReleased[0].ERABID != 6 {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

func TestERABReleaseResponseRoundTripsUserLocation(t *testing.T) {
	plmn := PLMNIdentity{0x00, 0xf1, 0x10}
	in := &ERABReleaseResponse{
		MMEUES1APID: 42,
		ENBUES1APID: 1,
		UserLocationInformation: &UserLocationInformation{
			EUTRANCGI: EUTRANCGI{PLMNIdentity: plmn, CellID: 0x0abcde1},
			TAI:       TAI{PLMNIdentity: plmn, TAC: 9},
		},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseERABReleaseResponse(pdu.(*SuccessfulOutcome).Value)
	if err != nil {
		t.Fatal(err)
	}

	uli := out.UserLocationInformation
	if uli == nil || uli.EUTRANCGI.CellID != 0x0abcde1 || uli.TAI.TAC != 9 {
		t.Fatalf("ULI not round-tripped: %+v", uli)
	}
}
