// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"reflect"
	"testing"
)

func TestPagingRoundtrip(t *testing.T) {
	in := &Paging{
		UEIdentityIndexValue: Ptr(uint16(0x2a9)), // 10-bit value (IMSI mod 1024)
		STMSI:                &STMSI{MMEC: 0x01, MTMSI: 0xdeadbeef},
		CNDomain:             Ptr(CNDomainPS),
		TAIList: []TAI{
			{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: TAC(0x0001)},
		},
		UERadioCapabilityForPaging: []byte{0xaa, 0xbb, 0xcc},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal pdu: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcPaging {
		t.Fatalf("expected Paging InitiatingMessage, got %T (proc %v)", pdu, im)
	}

	out, err := ParsePaging(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if deref(out.UEIdentityIndexValue) != deref(in.UEIdentityIndexValue) {
		t.Fatalf("UE identity index = %#x, want %#x", out.UEIdentityIndexValue, in.UEIdentityIndexValue)
	}

	if deref(out.STMSI) != deref(in.STMSI) {
		t.Fatalf("S-TMSI = %+v, want %+v", out.STMSI, in.STMSI)
	}

	if deref(out.CNDomain) != CNDomainPS {
		t.Fatalf("CN domain = %d, want PS", out.CNDomain)
	}

	if !reflect.DeepEqual(out.TAIList, in.TAIList) {
		t.Fatalf("TAI list = %+v, want %+v", out.TAIList, in.TAIList)
	}

	if !bytes.Equal(out.UERadioCapabilityForPaging, in.UERadioCapabilityForPaging) {
		t.Fatalf("UE Radio Capability for Paging = %x, want %x", out.UERadioCapabilityForPaging, in.UERadioCapabilityForPaging)
	}
}

// TestPagingOmitsRadioCapabilityForPaging verifies the optional IE is absent when no
// paging capability is set (it must not encode an empty octet string).
func TestPagingOmitsRadioCapabilityForPaging(t *testing.T) {
	in := &Paging{
		UEIdentityIndexValue: Ptr(uint16(0x2a9)),
		STMSI:                &STMSI{MMEC: 0x01, MTMSI: 0xdeadbeef},
		CNDomain:             Ptr(CNDomainPS),
		TAIList:              []TAI{{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: TAC(0x0001)}},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	out, err := ParsePaging(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.UERadioCapabilityForPaging != nil {
		t.Fatalf("expected no UE Radio Capability for Paging, got %x", out.UERadioCapabilityForPaging)
	}
}

// Paging DRX and Paging Priority are both optional-ignore IEs of the Paging
// container (TS 36.413 §9.1.6.1); the 5G side carries the same pair, so they
// must round-trip here too. Absent means the IE is omitted, not defaulted.
func TestPagingDRXAndPriorityRoundTrip(t *testing.T) {
	// UE Identity Index Value and CN Domain are mandatory in S1AP and have no
	// 5G counterpart (TS 36.413 §9.1.6.1).
	in := &Paging{
		UEIdentityIndexValue: Ptr(uint16(42)),
		STMSI:                &STMSI{MMEC: 1, MTMSI: 0xdeadbeef},
		PagingDRX:            Ptr(PagingDRXv128),
		CNDomain:             Ptr(CNDomainPS),
		TAIList:              []TAI{{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}},
		PagingPriority:       Ptr(PagingPriorityLevel3),
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParsePaging(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.PagingDRX == nil || *out.PagingDRX != PagingDRXv128 {
		t.Errorf("PagingDRX = %v, want v128", out.PagingDRX)
	}

	if out.PagingPriority == nil || *out.PagingPriority != PagingPriorityLevel3 {
		t.Errorf("PagingPriority = %v, want priolevel3", out.PagingPriority)
	}

	// Omitted stays omitted.
	in.PagingDRX, in.PagingPriority = nil, nil

	b, err = in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, _ = Unmarshal(b)

	out, err = ParsePaging(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.PagingDRX != nil || out.PagingPriority != nil {
		t.Errorf("absent IEs decoded to non-nil: DRX=%v priority=%v", out.PagingDRX, out.PagingPriority)
	}
}
