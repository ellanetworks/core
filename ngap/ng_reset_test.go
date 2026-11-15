// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// Golden NG RESET PDUs
const (
	goldenNGResetAll         = "0014000d000002000f4001860058000100"
	goldenNGResetPart        = "00140014000002000f40018600580008400260070009200b"
	goldenNGResetAcknowledge = "2014000c000001006f40050160070009"
)

func goldNGResetAll() *NGReset {
	return &NGReset{
		Cause:     Ptr(CauseMiscOMIntervention.Cause()),
		ResetType: ResetType{NGInterface: Ptr(ResetAllResetAll)},
	}
}

func goldNGResetPart() *NGReset {
	return &NGReset{
		Cause: Ptr(CauseMiscOMIntervention.Cause()),
		ResetType: ResetType{PartOfNGInterface: &UEAssociatedLogicalNGConnectionList{
			{AMFUENGAPID: Ptr(AMFUENGAPID(7)), RANUENGAPID: Ptr(RANUENGAPID(9))},
			// An item may carry either identity alone.
			{RANUENGAPID: Ptr(RANUENGAPID(11))},
		}},
	}
}

func goldNGResetAcknowledge() *NGResetAcknowledge {
	return &NGResetAcknowledge{
		ConnectionList: UEAssociatedLogicalNGConnectionList{
			{AMFUENGAPID: Ptr(AMFUENGAPID(7)), RANUENGAPID: Ptr(RANUENGAPID(9))},
		},
	}
}

func TestNGResetGoldenEncode(t *testing.T) {
	tests := []struct {
		name    string
		marshal func() ([]byte, error)
		want    string
	}{
		{"reset all", goldNGResetAll().Marshal, goldenNGResetAll},
		{"part of NG interface", goldNGResetPart().Marshal, goldenNGResetPart},
		{"acknowledge", goldNGResetAcknowledge().Marshal, goldenNGResetAcknowledge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.marshal()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			if want := mustHex(t, tt.want); !bytes.Equal(got, want) {
				t.Fatalf("encode mismatch:\n  got  %x\n  want %x", got, want)
			}
		})
	}
}

func TestNGResetGoldenDecode(t *testing.T) {
	tests := []struct {
		name string
		hex  string
		want *NGReset
	}{
		{"reset all", goldenNGResetAll, goldNGResetAll()},
		{"part of NG interface", goldenNGResetPart, goldNGResetPart()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdu, err := Unmarshal(mustHex(t, tt.hex))
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			im, ok := pdu.(*InitiatingMessage)
			if !ok || im.ProcedureCode != ProcNGReset {
				t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
			}

			msg, err := ParseNGReset(im.Value)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			if !deref(msg.Cause).Equal(deref(tt.want.Cause)) {
				t.Errorf("cause = %v, want %v", deref(msg.Cause), deref(tt.want.Cause))
			}

			if (msg.ResetType.NGInterface == nil) != (tt.want.ResetType.NGInterface == nil) {
				t.Fatalf("NGInterface = %v, want %v", msg.ResetType.NGInterface, tt.want.ResetType.NGInterface)
			}

			gotPart, wantPart := deref(msg.ResetType.PartOfNGInterface), deref(tt.want.ResetType.PartOfNGInterface)
			if len(gotPart) != len(wantPart) {
				t.Fatalf("Part = %+v, want %d items", gotPart, len(wantPart))
			}

			for i, item := range gotPart {
				w := wantPart[i]
				if deref(item.AMFUENGAPID) != deref(w.AMFUENGAPID) || deref(item.RANUENGAPID) != deref(w.RANUENGAPID) {
					t.Errorf("item %d = %+v, want %+v", i, item, w)
				}

				// An absent identity must stay absent, not read as id 0.
				if (item.AMFUENGAPID == nil) != (w.AMFUENGAPID == nil) {
					t.Errorf("item %d AMF id presence = %v, want %v", i, item.AMFUENGAPID, w.AMFUENGAPID)
				}
			}
		})
	}
}

func TestNGResetAcknowledgeGoldenDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenNGResetAcknowledge))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcNGReset {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	msg, err := ParseNGResetAcknowledge(so.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(msg.ConnectionList) != 1 ||
		deref(msg.ConnectionList[0].AMFUENGAPID) != 7 ||
		deref(msg.ConnectionList[0].RANUENGAPID) != 9 {
		t.Fatalf("connection list = %+v", msg.ConnectionList)
	}

	if msg.CriticalityDiagnostics != nil {
		t.Errorf("CriticalityDiagnostics = %+v, want nil", msg.CriticalityDiagnostics)
	}
}

// The acknowledge has no mandatory IE, so an empty one is valid.
func TestNGResetAcknowledgeEmpty(t *testing.T) {
	b, err := (&NGResetAcknowledge{}).Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseNGResetAcknowledge(pdu.value())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(msg.ConnectionList) != 0 || msg.CriticalityDiagnostics != nil {
		t.Errorf("empty acknowledge decoded to %+v", msg)
	}
}

func TestResetTypeChoiceExtensionIsPreserved(t *testing.T) {
	w := per.NewWriter()
	enc := per.Aligned

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, 2, 2); err != nil {
		t.Fatal(err)
	}

	if err := encodeContainerField(w, ieField{ID: 9999, Criticality: CriticalityReject, Value: apimpl.RawValue([]byte{0x00})}); err != nil {
		t.Fatal(err)
	}

	wire := perBytes(w)

	got, err := unmarshalPERValue[ResetType](wire)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.NGInterface != nil || got.PartOfNGInterface != nil || got.ChoiceExtension == nil {
		t.Fatalf("ResetType = %+v, want only the raw choice-Extensions", got)
	}

	if d := ap.Diagnose[ProtocolIEID](&got); len(d.IEs) != 1 || d.IEs[0].Criticality != CriticalityReject {
		t.Errorf("diagnostics = %+v", d.IEs)
	}

	w2 := per.NewWriter()
	if err := got.MarshalPER(w2, enc); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(perBytes(w2), wire) {
		t.Errorf("re-encode = % x, want % x", perBytes(w2), wire)
	}
}

// A reset naming no UE association at all is refused: the list is
// SIZE(1..maxnoofNGConnectionsToReset), so an empty one cannot be encoded.
func TestResetTypeRejectsEmptyPart(t *testing.T) {
	w := per.NewWriter()
	if err := (&ResetType{PartOfNGInterface: &UEAssociatedLogicalNGConnectionList{}}).MarshalPER(w, per.Aligned); err == nil {
		t.Fatal("encode accepted an empty partOfNG-Interface list")
	}
}
