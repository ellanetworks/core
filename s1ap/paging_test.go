// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

func TestPagingRoundtrip(t *testing.T) {
	in := &Paging{
		UEIdentityIndexValue: Ptr(uint16(0x2a9)), // 10-bit value (IMSI mod 1024)
		UEPagingID:           &UEPagingID{STMSI: &STMSI{MMEC: 0x01, MTMSI: 0xdeadbeef}},
		CNDomain:             Ptr(CNDomainPS),
		TAIList: TAIList{
			{TAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: TAC(0x0001)}},
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

	if !out.UEPagingID.Equal(*in.UEPagingID) {
		t.Fatalf("UE Paging ID = %+v, want %+v", out.UEPagingID, in.UEPagingID)
	}

	if deref(out.CNDomain) != CNDomainPS {
		t.Fatalf("CN domain = %d, want PS", out.CNDomain)
	}

	if !out.TAIList.Equal(in.TAIList) {
		t.Fatalf("TAI list = %+v, want %+v", out.TAIList, in.TAIList)
	}

	if !bytes.Equal(out.UERadioCapabilityForPaging, in.UERadioCapabilityForPaging) {
		t.Fatalf("UE Radio Capability for Paging = %x, want %x", out.UERadioCapabilityForPaging, in.UERadioCapabilityForPaging)
	}
}

// An unset optional IE must be omitted, not encoded as an empty octet string.
func TestPagingOmitsRadioCapabilityForPaging(t *testing.T) {
	in := &Paging{
		UEIdentityIndexValue: Ptr(uint16(0x2a9)),
		UEPagingID:           &UEPagingID{STMSI: &STMSI{MMEC: 0x01, MTMSI: 0xdeadbeef}},
		CNDomain:             Ptr(CNDomainPS),
		TAIList:              TAIList{{TAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: TAC(0x0001)}}},
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

// Both optional-ignore (§9.1.6). Absent means omitted, not defaulted.
func TestPagingDRXAndPriorityRoundTrip(t *testing.T) {
	// Mandatory in S1AP, with no 5G counterpart.
	in := &Paging{
		UEIdentityIndexValue: Ptr(uint16(42)),
		UEPagingID:           &UEPagingID{STMSI: &STMSI{MMEC: 1, MTMSI: 0xdeadbeef}},
		PagingDRX:            Ptr(PagingDRXv128),
		CNDomain:             Ptr(CNDomainPS),
		TAIList:              TAIList{{TAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}}},
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

// §9.2.3.13 makes UE Paging ID an extensible CHOICE of S-TMSI and IMSI. Both
// root alternatives decode, and an extension alternative is kept verbatim and
// reported on the IE's ignore criticality, so §10.3.4.2 carries on with the
// message.
func TestUEPagingIDAlternatives(t *testing.T) {
	taiList := &TAIList{{TAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}}}

	cases := []struct {
		name string
		id   []byte
		want UEPagingID
	}{
		{"IMSI alternative", []byte{0x40, 0x21, 0x43, 0x65}, UEPagingID{IMSI: &IMSI{0x21, 0x43, 0x65}}},
		{"extension alternative", []byte{0x80, 0x01, 0x2a}, UEPagingID{Ext: &per.ExtensionAlternative{Index: 0, Value: "\x2a"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := DecodePaging(container(t,
				ieField{ID: IDUEPagingID, Criticality: CriticalityIgnore, Value: ap.RawValue(tc.id)},
				ieField{ID: IDTAIList, Criticality: CriticalityIgnore, Value: taiList},
			))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			if len(msg.TAIList) != 1 {
				t.Errorf("rest of the message lost: %+v", msg.TAIList)
			}

			if tc.want.Ext != nil {
				if msg.UEPagingID != nil {
					t.Fatalf("UEPagingID = %+v, want it kept raw", msg.UEPagingID)
				}

				if err := msg.Diagnostics().Check(); err != nil {
					t.Fatalf("an ignore IE must not fail the check: %v", err)
				}
			} else if msg.UEPagingID == nil || !msg.UEPagingID.Equal(tc.want) {
				t.Fatalf("UEPagingID = %+v, want %+v", msg.UEPagingID, tc.want)
			}

			out, err := msg.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			pdu, err := Unmarshal(out)
			if err != nil {
				t.Fatal(err)
			}

			if got := pdu.(*InitiatingMessage).Value; !bytes.Equal(got, container(t,
				ieField{ID: IDUEPagingID, Criticality: CriticalityIgnore, Value: ap.RawValue(tc.id)},
				ieField{ID: IDTAIList, Criticality: CriticalityIgnore, Value: taiList},
			)) {
				t.Fatalf("re-encoded body % x", got)
			}
		})
	}
}

// The TAI list is SIZE(1..maxnoofTAIs), so an empty one cannot be encoded and an
// over-long one must be refused.
func TestPagingTAIListBounds(t *testing.T) {
	base := func() *Paging {
		return &Paging{
			UEIdentityIndexValue: Ptr(uint16(42)),
			UEPagingID:           &UEPagingID{STMSI: &STMSI{MMEC: 1, MTMSI: 0xdeadbeef}},
			CNDomain:             Ptr(CNDomainPS),
		}
	}

	m := base()
	if _, err := m.Marshal(); err == nil {
		t.Error("encoded a Paging with no TAI")
	}

	m = base()
	m.TAIList = make(TAIList, maxnoofTAIs+1)

	for i := range m.TAIList {
		m.TAIList[i] = TAIItem{TAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: TAC(i)}}
	}

	if _, err := m.Marshal(); err == nil {
		t.Fatalf("encoded %d TAIs, want a bound error", maxnoofTAIs+1)
	}
}
