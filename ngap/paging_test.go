// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// Golden PAGING PDUs
const (
	goldenPagingMinimal = "0018402000000200734007000820deadbeef0067400e1000f1100000010000f110000002"
	goldenPagingFull    = "0018403700000600734007000820deadbeef00324001400067400e1000f1100000010000f1100000020034400120007640044002aabb0033400100"
)

func goldPagingMinimal() *Paging {
	return &Paging{
		UEPagingIdentity: &UEPagingIdentity{FiveGSTMSI: &FiveGSTMSI{AMFSetID: 1, AMFPointer: 1, FiveGTMSI: 0xdeadbeef}},
		TAIListForPaging: TAIListForPaging{
			{TAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}},
			{TAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 2}},
		},
	}
}

func goldPagingFull() *Paging {
	m := goldPagingMinimal()
	m.PagingDRX = Ptr(PagingDRXv128)
	m.PagingPriority = Ptr(PagingPriorityLevel3)
	m.UERadioCapabilityForPaging = &UERadioCapabilityForPaging{
		NR: Ptr(UERadioCapabilityForPagingOfNR{0xaa, 0xbb}),
	}
	m.PagingOrigin = Ptr(PagingOriginNon3GPP)

	return m
}

func TestPagingGoldenEncode(t *testing.T) {
	tests := []struct {
		name string
		msg  *Paging
		want string
	}{
		{"minimal", goldPagingMinimal(), goldenPagingMinimal},
		{"every modeled IE", goldPagingFull(), goldenPagingFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.msg.Marshal()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			if want := mustHex(t, tt.want); !bytes.Equal(got, want) {
				t.Fatalf("encode mismatch:\n  got  %x\n  want %x", got, want)
			}
		})
	}
}

func TestPagingGoldenDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenPagingFull))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcPaging {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	msg, err := ParsePaging(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := goldPagingFull()

	if msg.UEPagingIdentity == nil || !msg.UEPagingIdentity.Equal(*want.UEPagingIdentity) {
		t.Errorf("UEPagingIdentity = %+v, want %+v", msg.UEPagingIdentity, want.UEPagingIdentity)
	}

	if len(msg.TAIListForPaging) != 2 ||
		msg.TAIListForPaging[0].TAI.TAC != 1 || msg.TAIListForPaging[1].TAI.TAC != 2 {
		t.Fatalf("TAIListForPaging = %+v", msg.TAIListForPaging)
	}

	if deref(msg.PagingDRX) != PagingDRXv128 {
		t.Errorf("PagingDRX = %d", deref(msg.PagingDRX))
	}

	if deref(msg.PagingPriority) != PagingPriorityLevel3 {
		t.Errorf("PagingPriority = %d", deref(msg.PagingPriority))
	}

	if deref(msg.PagingOrigin) != PagingOriginNon3GPP {
		t.Errorf("PagingOrigin = %d", deref(msg.PagingOrigin))
	}

	if msg.UERadioCapabilityForPaging == nil || msg.UERadioCapabilityForPaging.NR == nil ||
		!bytes.Equal(*msg.UERadioCapabilityForPaging.NR, UERadioCapabilityForPagingOfNR{0xaa, 0xbb}) {
		t.Errorf("UERadioCapabilityForPaging = %+v", msg.UERadioCapabilityForPaging)
	}

	// Only the NR capability was sent, so the E-UTRA one must stay absent.
	if msg.UERadioCapabilityForPaging != nil && msg.UERadioCapabilityForPaging.EUTRA != nil {
		t.Errorf("EUTRA capability = %x, want nil", *msg.UERadioCapabilityForPaging.EUTRA)
	}
}

// Every Paging IE is ignore criticality, so §10.3.5 still delivers a message
// missing a mandatory one. §9.1.1 still obliges the sender.
func TestPagingMissingMandatoryIEs(t *testing.T) {
	if _, err := (&Paging{}).Marshal(); err == nil {
		t.Error("Marshal() = nil error, want a required-IE error")
	}

	msg, err := ParsePaging(container(t,
		ieField{ID: IDPagingDRX, Criticality: CriticalityIgnore, Value: PagingDRXv128},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.UEPagingIdentity != nil || msg.TAIListForPaging != nil {
		t.Errorf("absent IEs decoded to non-nil: %+v", msg)
	}

	var missing int

	for _, ie := range msg.Diagnostics().IEs {
		if typeOfError(ie) == TypeOfErrorMissing &&
			(ie.ID == IDUEPagingIdentity || ie.ID == IDTAIListForPaging) {
			missing++
		}
	}

	if missing != 2 {
		t.Errorf("reported %d missing mandatory IEs, want 2: %+v", missing, msg.Diagnostics().IEs)
	}
}

func TestUEPagingIdentityChoiceExtensionIsPreserved(t *testing.T) {
	w := per.NewWriter()
	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0,
		uePagingIdentityAlternatives-1, uePagingIdentityChoiceExtensions); err != nil {
		t.Fatal(err)
	}

	if err := encodeContainerField(w, ieField{
		ID: 9999, Criticality: CriticalityIgnore, Value: apimpl.RawValue([]byte{0x00}),
	}); err != nil {
		t.Fatal(err)
	}

	value := container(t,
		ieField{ID: IDUEPagingIdentity, Criticality: CriticalityIgnore, Value: apimpl.RawValue(perBytes(w))},
		ieField{ID: IDTAIListForPaging, Criticality: CriticalityIgnore, Value: &TAIListForPaging{
			{TAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}},
		}},
	)

	parsed, err := ParsePaging(value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if parsed.UEPagingIdentity != nil {
		t.Errorf("parsed UEPagingIdentity = %+v, want it treated as not received", parsed.UEPagingIdentity)
	}

	msg, err := DecodePaging(value)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if msg.UEPagingIdentity == nil || msg.UEPagingIdentity.ChoiceExtension == nil || msg.UEPagingIdentity.FiveGSTMSI != nil {
		t.Errorf("UEPagingIdentity = %+v, want the raw choice-Extensions kept", msg.UEPagingIdentity)
	}

	if len(msg.TAIListForPaging) != 1 {
		t.Errorf("rest of the message lost: %+v", msg.TAIListForPaging)
	}

	b, err := msg.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(pdu.value(), value) {
		t.Errorf("re-encode = % x, want % x", pdu.value(), value)
	}
}

func TestTAIListForPagingItemKeepsItemExtensions(t *testing.T) {
	tai := TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 7}
	item := TAIListForPagingItem{TAI: tai}
	item.iEExtensions.Append(rawIE{ID: 9999, Criticality: CriticalityIgnore, Value: []byte{0x01, 0x02}})

	in := goldPagingMinimal()
	in.TAIListForPaging = TAIListForPaging{item}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParsePaging(pdu.value())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(msg.TAIListForPaging) != 1 || msg.TAIListForPaging[0].TAI != tai {
		t.Fatalf("TAIListForPaging = %+v", msg.TAIListForPaging)
	}

	unknown := msg.TAIListForPaging[0].iEExtensions.UnknownIEs()
	if len(unknown) != 1 || unknown[0].ID != 9999 || !bytes.Equal(unknown[0].Value, []byte{0x01, 0x02}) {
		t.Fatalf("item extensions = %+v, want the unknown IE kept", unknown)
	}

	if !msg.TAIListForPaging.Equal(in.TAIListForPaging) {
		t.Errorf("decoded list %+v, want %+v", msg.TAIListForPaging, in.TAIListForPaging)
	}

	re, err := msg.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(re, b) {
		t.Errorf("re-encode = % x, want % x", re, b)
	}
}

// The list is SIZE(1..maxnoofTAIforPaging), so an empty one cannot be encoded
// and an over-long one must be refused.
func TestPagingTAIListBounds(t *testing.T) {
	m := goldPagingMinimal()
	m.TAIListForPaging = nil

	if _, err := m.Marshal(); err == nil {
		t.Error("encoded a Paging with no TAI for paging")
	}

	m.TAIListForPaging = make(TAIListForPaging, maxnoofTAIforPaging+1)
	for i := range m.TAIListForPaging {
		m.TAIListForPaging[i] = TAIListForPagingItem{TAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: TAC(i)}}
	}

	_, err := m.Marshal()
	if err == nil {
		t.Fatalf("encoded %d TAIs, want a bound error", maxnoofTAIforPaging+1)
	}

	if !strings.Contains(err.Error(), "range") {
		t.Errorf("error = %q, want it to name the bound", err)
	}
}
