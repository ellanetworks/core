// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
)

// Golden PAGING PDUs
const (
	goldenPagingMinimal = "0018402000000200734007000820deadbeef0067400e1000f1100000010000f110000002"
	goldenPagingFull    = "0018403700000600734007000820deadbeef00324001400067400e1000f1100000010000f1100000020034400120007640044002aabb0033400100"
)

func goldPagingMinimal() *Paging {
	return &Paging{
		FiveGSTMSI: &FiveGSTMSI{AMFSetID: 1, AMFPointer: 1, FiveGTMSI: 0xdeadbeef},
		TAIListForPaging: []TAI{
			{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
			{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 2},
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

	if msg.FiveGSTMSI == nil || *msg.FiveGSTMSI != *want.FiveGSTMSI {
		t.Errorf("FiveGSTMSI = %+v, want %+v", msg.FiveGSTMSI, want.FiveGSTMSI)
	}

	if len(msg.TAIListForPaging) != 2 ||
		msg.TAIListForPaging[0].TAC != 1 || msg.TAIListForPaging[1].TAC != 2 {
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
		ieField{id: IDPagingDRX, crit: CriticalityIgnore, val: PagingDRXv128},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.FiveGSTMSI != nil || msg.TAIListForPaging != nil {
		t.Errorf("absent IEs decoded to non-nil: %+v", msg)
	}

	var missing int

	for _, ie := range msg.Diagnostics().IEs {
		if ie.TypeOfError == TypeOfErrorMissing &&
			(ie.ID == IDUEPagingIdentity || ie.ID == IDTAIListForPaging) {
			missing++
		}
	}

	if missing != 2 {
		t.Errorf("reported %d missing mandatory IEs, want 2: %+v", missing, msg.Diagnostics().IEs)
	}
}

// §9.3.3.18 closes UEPagingIdentity with a choice-Extensions alternative, so
// selecting it must error rather than yield a zero identity.
func TestUEPagingIdentityChoiceExtensionIsRejected(t *testing.T) {
	w := per.NewWriter()
	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0,
		uePagingIdentityAlternatives-1, uePagingIdentityChoiceExtensions); err != nil {
		t.Fatal(err)
	}

	if err := encodeContainerField(w, per.Aligned, ieField{
		id: IDUEPagingIdentity, crit: CriticalityReject, raw: []byte{0x00},
	}); err != nil {
		t.Fatal(err)
	}

	value := container(t,
		ieField{id: IDUEPagingIdentity, crit: CriticalityIgnore, raw: perBytes(w)},
		ieField{id: IDTAIListForPaging, crit: CriticalityIgnore, val: per.MarshalerFunc(
			func(w *per.Writer, enc per.Encoding) error {
				return marshalSeqOf(w, enc, 1, maxnoofTAIforPaging, []taiListForPagingItem{
					{TAI: TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1}},
				})
			})},
	)

	// Ignore criticality, so §10.3.4.2 drops the content and carries on.
	msg, err := ParsePaging(value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.FiveGSTMSI != nil {
		t.Errorf("FiveGSTMSI = %+v, want nil for an unsupported alternative", msg.FiveGSTMSI)
	}

	if len(msg.TAIListForPaging) != 1 {
		t.Errorf("rest of the message lost: %+v", msg.TAIListForPaging)
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

	m.TAIListForPaging = make([]TAI, maxnoofTAIforPaging+1)
	for i := range m.TAIListForPaging {
		m.TAIListForPaging[i] = TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: TAC(i)}
	}

	_, err := m.Marshal()
	if err == nil {
		t.Fatalf("encoded %d TAIs, want a bound error", maxnoofTAIforPaging+1)
	}

	if !strings.Contains(err.Error(), "range") {
		t.Errorf("error = %q, want it to name the bound", err)
	}
}
