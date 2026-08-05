// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"errors"
	"testing"
)

// Golden UE RADIO CAPABILITY INFO INDICATION PDUs.
const (
	goldenUERadioCapabilityInfoIndication     = "002c4017000003000a000200010055000200020075400403010203"
	goldenUERadioCapabilityInfoIndicationFull = "002c4020000004000a0002000100550002000200754004030102030076400560010a010b"
)

func goldUERadioCapabilityInfoIndication() *UERadioCapabilityInfoIndication {
	return &UERadioCapabilityInfoIndication{
		AMFUENGAPID:       1,
		RANUENGAPID:       2,
		UERadioCapability: UERadioCapability{0x01, 0x02, 0x03},
	}
}

func TestUERadioCapabilityInfoIndicationGoldenEncodings(t *testing.T) {
	full := goldUERadioCapabilityInfoIndication()
	full.UERadioCapabilityForPaging = &UERadioCapabilityForPaging{
		NR:    &UERadioCapabilityForPagingOfNR{0x0a},
		EUTRA: &UERadioCapabilityForPagingOfEUTRA{0x0b},
	}

	tests := []struct {
		name string
		msg  interface{ Marshal() ([]byte, error) }
		want string
	}{
		{"Indication", goldUERadioCapabilityInfoIndication(), goldenUERadioCapabilityInfoIndication},
		{"IndicationWithPaging", full, goldenUERadioCapabilityInfoIndicationFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mustMarshalHex(t, tt.msg); got != tt.want {
				t.Errorf("encoded\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestUERadioCapabilityInfoIndicationRoundTrips(t *testing.T) {
	in := &UERadioCapabilityInfoIndication{
		AMFUENGAPID:       0x020000bf,
		RANUENGAPID:       1,
		UERadioCapability: UERadioCapability{0x01, 0x02, 0x03, 0x04, 0x05},
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

	if init.ProcedureCode != ProcUERadioCapabilityInfoIndication {
		t.Fatalf("procedure code: expected %d, got %d", ProcUERadioCapabilityInfoIndication, init.ProcedureCode)
	}

	out, err := ParseUERadioCapabilityInfoIndication(init.Value)
	if err != nil {
		t.Fatalf("ParseUERadioCapabilityInfoIndication: %v", err)
	}

	if out.AMFUENGAPID != in.AMFUENGAPID || out.RANUENGAPID != in.RANUENGAPID ||
		!bytes.Equal(out.UERadioCapability, in.UERadioCapability) {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

// NGAP carries the NR and E-UTRA paging capabilities separately, where S1AP
// carries one opaque string (§9.3.1.68).
func TestUERadioCapabilityInfoIndicationPaging(t *testing.T) {
	in := goldUERadioCapabilityInfoIndication()
	in.UERadioCapabilityForPaging = &UERadioCapabilityForPaging{
		NR:    &UERadioCapabilityForPagingOfNR{0xaa, 0xbb},
		EUTRA: &UERadioCapabilityForPagingOfEUTRA{0xcc},
	}

	raw, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(raw)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseUERadioCapabilityInfoIndication(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.UERadioCapabilityForPaging == nil {
		t.Fatal("UERadioCapabilityForPaging absent")
	}

	if !bytes.Equal(*out.UERadioCapabilityForPaging.NR, *in.UERadioCapabilityForPaging.NR) ||
		!bytes.Equal(*out.UERadioCapabilityForPaging.EUTRA, *in.UERadioCapabilityForPaging.EUTRA) {
		t.Fatalf("paging capability = %+v, want %+v", out.UERadioCapabilityForPaging, in.UERadioCapabilityForPaging)
	}
}

// The UE Radio Capability is mandatory but ignore criticality, so a message
// without it is still delivered and the absence reported (§10.3.5).
func TestUERadioCapabilityInfoIndicationWithoutCapability(t *testing.T) {
	value := container(t,
		ieField{id: idAMFUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(AMFUENGAPID(1)))},
		ieField{id: idRANUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(RANUENGAPID(2)))},
	)

	msg, err := ParseUERadioCapabilityInfoIndication(value)
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

// Both UE NGAP IDs are reject criticality, so the AMF cannot act on a message
// missing one (§10.3.5).
func TestUERadioCapabilityInfoIndicationMissingRejectIE(t *testing.T) {
	value := container(t,
		ieField{id: idAMFUENGAPID, crit: CriticalityReject, raw: ieRaw(t, Ptr(AMFUENGAPID(1)))},
		ieField{id: idUERadioCapability, crit: CriticalityIgnore, raw: ieRaw(t, UERadioCapability{0x01})},
	)

	_, err := ParseUERadioCapabilityInfoIndication(value)
	if err == nil {
		t.Fatal("parse succeeded without the RAN UE NGAP ID")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != idRANUENGAPID ||
		ase.IEs[0].TypeOfError != TypeOfErrorMissing {
		t.Errorf("diagnostics = %+v, want one missing entry for RANUENGAPID", ase.IEs)
	}
}
