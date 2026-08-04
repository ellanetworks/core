// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"
)

// Golden AMF STATUS INDICATION PDUs
const (
	goldenAMFStatusIndication     = "0001400f00000100780008000000f110cafe00"
	goldenAMFStatusIndicationFull = "0001401b00000100780014006000f110cafe0002406261636b75702d616d66"
)

func goldGUAMI() GUAMI {
	return GUAMI{
		PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
		AMFRegionID:  0xca,
		AMFSetID:     0x3f8, // 0xfe00 >> 6, the top 10 bits
		AMFPointer:   0,
	}
}

func TestAMFStatusIndicationGoldenEncode(t *testing.T) {
	tests := []struct {
		name string
		msg  *AMFStatusIndication
		want string
	}{
		{
			"minimal",
			&AMFStatusIndication{UnavailableGUAMIList: UnavailableGUAMIList{{GUAMI: goldGUAMI()}}},
			goldenAMFStatusIndication,
		},
		{
			"every modeled IE",
			&AMFStatusIndication{UnavailableGUAMIList: UnavailableGUAMIList{{
				GUAMI:                        goldGUAMI(),
				TimerApproachForGUAMIRemoval: Ptr(TimerApproachApplyTimer),
				BackupAMFName:                Ptr(Name("backup-amf")),
			}}},
			goldenAMFStatusIndicationFull,
		},
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

func TestAMFStatusIndicationGoldenDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenAMFStatusIndicationFull))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcAMFStatusIndication {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	msg, err := ParseAMFStatusIndication(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(msg.UnavailableGUAMIList) != 1 {
		t.Fatalf("UnavailableGUAMIList = %+v, want one item", msg.UnavailableGUAMIList)
	}

	item := msg.UnavailableGUAMIList[0]
	if item.GUAMI != goldGUAMI() {
		t.Errorf("GUAMI = %+v, want %+v", item.GUAMI, goldGUAMI())
	}

	if deref(item.TimerApproachForGUAMIRemoval) != TimerApproachApplyTimer {
		t.Errorf("TimerApproachForGUAMIRemoval = %v", item.TimerApproachForGUAMIRemoval)
	}

	if deref(item.BackupAMFName) != Name("backup-amf") {
		t.Errorf("BackupAMFName = %q", deref(item.BackupAMFName))
	}
}

// Both per-item IEs are optional: absent must stay absent rather than decode as
// a zero that reads as "apply timer" or an empty backup name.
func TestAMFStatusIndicationOptionalsStayAbsent(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenAMFStatusIndication))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseAMFStatusIndication(pdu.value())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	item := msg.UnavailableGUAMIList[0]
	if item.TimerApproachForGUAMIRemoval != nil || item.BackupAMFName != nil {
		t.Errorf("absent IEs decoded to non-nil: %+v", item)
	}
}

// The Unavailable GUAMI List is mandatory with reject criticality, so §9.1.1
// obliges the sender and §10.3.4.2 has the receiver reject the procedure rather
// than act on a message naming no GUAMI.
func TestAMFStatusIndicationRequiresGUAMIList(t *testing.T) {
	if _, err := (&AMFStatusIndication{}).Marshal(); err == nil {
		t.Error("Marshal() = nil error, want a required-IE error")
	}

	if _, err := ParseAMFStatusIndication(container(t)); err == nil {
		t.Error("decoded an AMF Status Indication with no Unavailable GUAMI List")
	}
}

// The list is SIZE(1..maxnoofServedGUAMIs); neither bound may be crossed.
func TestAMFStatusIndicationGUAMIListBounds(t *testing.T) {
	over := make(UnavailableGUAMIList, maxnoofServedGUAMIs+1)
	for i := range over {
		over[i] = UnavailableGUAMIItem{GUAMI: goldGUAMI()}
	}

	if _, err := (&AMFStatusIndication{UnavailableGUAMIList: over}).Marshal(); err == nil {
		t.Fatalf("encoded %d GUAMIs, want a bound error", len(over))
	}
}
