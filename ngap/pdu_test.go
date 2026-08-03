// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"
)

func TestEncodePDUKnownVector(t *testing.T) {
	// A real NGSetupRequest PDU begins 00 15 (initiatingMessage, procedureCode
	// 21). Here the open-type value is a single placeholder octet.
	got, err := Marshal(&InitiatingMessage{
		ProcedureCode: ProcNGSetup,
		Criticality:   CriticalityReject,
		Value:         []byte{0xab},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []byte{0x00, 0x15, 0x00, 0x01, 0xab}
	if !bytes.Equal(got, want) {
		t.Fatalf("encoded % x, want % x", got, want)
	}
}

func TestPDURoundTrip(t *testing.T) {
	value := []byte{0xde, 0xad, 0xbe, 0xef}

	cases := []PDU{
		&InitiatingMessage{ProcNGSetup, CriticalityReject, value},
		&SuccessfulOutcome{ProcNGSetup, CriticalityReject, value},
		&UnsuccessfulOutcome{ProcNGSetup, CriticalityIgnore, value},
		&InitiatingMessage{ProcInitialUEMessage, CriticalityIgnore, []byte{0x01}},
	}
	for _, in := range cases {
		b, err := Marshal(in)
		if err != nil {
			t.Fatalf("%T: encode: %v", in, err)
		}

		out, err := Unmarshal(b)
		if err != nil {
			t.Fatalf("%T: decode: %v", in, err)
		}

		if out.choiceIndex() != in.choiceIndex() ||
			out.procedureCode() != in.procedureCode() ||
			out.criticality() != in.criticality() ||
			!bytes.Equal(out.value(), in.value()) {
			t.Fatalf("%T: round-trip mismatch: got %+v", in, out)
		}
	}
}

func TestDecodePDUFields(t *testing.T) {
	out, err := Unmarshal([]byte{0x00, 0x15, 0x00, 0x01, 0xab})
	if err != nil {
		t.Fatal(err)
	}

	im, ok := out.(*InitiatingMessage)
	if !ok {
		t.Fatalf("got %T, want *InitiatingMessage", out)
	}

	if im.ProcedureCode != ProcNGSetup || im.Criticality != CriticalityReject {
		t.Fatalf("fields: %+v", im)
	}

	if !bytes.Equal(im.Value, []byte{0xab}) {
		t.Fatalf("value % x", im.Value)
	}
}

func TestEncodePDUNil(t *testing.T) {
	if _, err := Marshal(nil); err == nil {
		t.Fatal("expected error for nil PDU")
	}
}

func TestDecodePDUTruncated(t *testing.T) {
	for _, b := range [][]byte{nil, {0x00}, {0x00, 0x15}, {0x00, 0x15, 0x00}} {
		if _, err := Unmarshal(b); err == nil {
			t.Fatalf("expected error for truncated input % x", b)
		}
	}
}

func TestCriticalityString(t *testing.T) {
	if CriticalityReject.String() != "reject" || CriticalityNotify.String() != "notify" {
		t.Fatal("unexpected Criticality string")
	}
}
