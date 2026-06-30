// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"
)

func TestUECapabilityInfoIndicationRoundTrips(t *testing.T) {
	in := &UECapabilityInfoIndication{
		MMEUES1APID:       0x020000bf,
		ENBUES1APID:       1,
		UERadioCapability: []byte{0x01, 0x02, 0x03, 0x04, 0x05},
	}

	raw, err := in.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	pdu, err := Unmarshal(raw)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	init, ok := pdu.(*InitiatingMessage)
	if !ok {
		t.Fatalf("expected *InitiatingMessage, got %T", pdu)
	}

	if init.ProcedureCode != ProcUECapabilityInfoIndication {
		t.Fatalf("procedure code: expected %d, got %d", ProcUECapabilityInfoIndication, init.ProcedureCode)
	}

	out, err := ParseUECapabilityInfoIndication(init.Value)
	if err != nil {
		t.Fatalf("ParseUECapabilityInfoIndication: %v", err)
	}

	if out.MMEUES1APID != in.MMEUES1APID || out.ENBUES1APID != in.ENBUES1APID ||
		!bytes.Equal(out.UERadioCapability, in.UERadioCapability) {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

// TestUECapabilityInfoIndicationPaging checks the optional UE Radio Capability
// for Paging IE (TS 36.413) round-trips when present.
func TestUECapabilityInfoIndicationPaging(t *testing.T) {
	in := &UECapabilityInfoIndication{
		MMEUES1APID:                1,
		ENBUES1APID:                2,
		UERadioCapability:          []byte{0x01, 0x02},
		UERadioCapabilityForPaging: []byte{0xaa, 0xbb, 0xcc},
	}

	raw, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseUECapabilityInfoIndication(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out.UERadioCapabilityForPaging, in.UERadioCapabilityForPaging) {
		t.Fatalf("UERadioCapabilityForPaging = %x, want %x", out.UERadioCapabilityForPaging, in.UERadioCapabilityForPaging)
	}
}
