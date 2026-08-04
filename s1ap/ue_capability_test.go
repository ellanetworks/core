// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"errors"
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

// The UE Radio Capability is mandatory but ignore criticality, so a message
// without it is still delivered and the absence reported (§10.3.5).
func TestUECapabilityInfoIndicationWithoutCapability(t *testing.T) {
	value := container(t,
		ieField{id: idMMEUES1APID, crit: CriticalityReject, raw: ieRaw(t, Ptr(MMEUES1APID(1)))},
		ieField{id: idENBUES1APID, crit: CriticalityReject, raw: ieRaw(t, Ptr(ENBUES1APID(2)))},
	)

	msg, err := ParseUECapabilityInfoIndication(value)
	if err != nil {
		t.Fatalf("an absent ignore-criticality IE must still deliver the message: %v", err)
	}

	if msg.UERadioCapability != nil {
		t.Errorf("UERadioCapability = %x, want nil", msg.UERadioCapability)
	}

	d := msg.Diagnostics()
	if len(d.IEs) != 1 || d.IEs[0].ID != idUERadioCapability || d.IEs[0].TypeOfError != TypeOfErrorMissing {
		t.Errorf("diagnostics = %+v, want one missing entry for UERadioCapability", d.IEs)
	}
}

// Both UE S1AP IDs are reject criticality, so the MME cannot act on a message
// missing one (§10.3.5).
func TestUECapabilityInfoIndicationMissingRejectIE(t *testing.T) {
	value := container(t,
		ieField{id: idMMEUES1APID, crit: CriticalityReject, raw: ieRaw(t, Ptr(MMEUES1APID(1)))},
		ieField{id: idUERadioCapability, crit: CriticalityIgnore, raw: ieRaw(t, UERadioCapability{0x01})},
	)

	_, err := ParseUECapabilityInfoIndication(value)
	if err == nil {
		t.Fatal("parse succeeded without the eNB UE S1AP ID")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != idENBUES1APID ||
		ase.IEs[0].TypeOfError != TypeOfErrorMissing {
		t.Errorf("diagnostics = %+v, want one missing entry for ENBUES1APID", ase.IEs)
	}
}
