// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

func testNgENBNodeID(t *testing.T, plmn PLMNIdentity, id uint32, bits int) GlobalRANNodeID {
	t.Helper()

	n, ok := NgENBIDOfBits(id, bits)
	if !ok {
		t.Fatalf("no ng-eNB identity is %d bits wide", bits)
	}

	return NewGlobalNgENBID(plmn, n)
}

func TestGlobalRANNodeIDRoundTrip(t *testing.T) {
	tngf := TNGFIDValue{BitStringFromUint(0xdeadbeef, 32)}
	wagf := WAGFIDValue{BitStringFrom([]byte{0xab, 0xcd, 0xe0}, 20)}

	for _, g := range []GlobalRANNodeID{
		NewGlobalGNBID(goldPLMN(), 0x000102, 24),
		NewGlobalGNBID(goldPLMN(), 0x3fffff, 22),
		NewGlobalGNBID(goldPLMN(), 0xffffffff, 32),
		testNgENBNodeID(t, goldPLMN(), 0xabcde, 20),
		testNgENBNodeID(t, goldPLMN(), 0x3ffff, 18),
		testNgENBNodeID(t, goldPLMN(), 0x1abcde, 21),
		NewGlobalN3IWFID(goldPLMN(), 0xbeef),
		{GlobalTNGFID: &GlobalTNGFID{PLMNIdentity: goldPLMN(), TNGFID: TNGFID{TNGFID: &tngf}}},
		{GlobalWAGFID: &GlobalWAGFID{PLMNIdentity: goldPLMN(), WAGFID: WAGFID{WAGFID: &wagf}}},
	} {
		w := per.NewWriter()
		if err := g.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("%+v: marshal: %v", g, err)
		}

		got, err := unmarshalPERValue[GlobalRANNodeID](perBytes(w))
		if err != nil {
			t.Fatalf("%+v: unmarshal: %v", g, err)
		}

		if !got.Equal(g) {
			t.Errorf("round trip = %+v, want %+v", got, g)
		}

		w2 := per.NewWriter()
		if err := got.MarshalPER(w2, per.Aligned); err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(perBytes(w), perBytes(w2)) {
			t.Errorf("re-encode = % x, want % x", perBytes(w2), perBytes(w))
		}
	}
}

func TestGlobalRANNodeIDTypedChoiceExtensionWireBytes(t *testing.T) {
	v := TWIFIDValue{BitStringFromUint(0x01020304, 32)}
	g := GlobalRANNodeID{GlobalTWIFID: &GlobalTWIFID{PLMNIdentity: goldPLMN(), TWIFID: TWIFID{TWIFID: &v}}}

	w := per.NewWriter()
	if err := g.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	inner, err := per.Marshal(g.GlobalTWIFID)
	if err != nil {
		t.Fatal(err)
	}

	want := per.NewWriter()
	if err := per.EncodeConstrainedWholeNumber(want, per.Aligned, 0, 3, 3); err != nil {
		t.Fatal(err)
	}

	if err := encodeContainerField(want, ieField{ID: IDGlobalTWIFID, Criticality: CriticalityReject, Value: ap.RawValue(inner)}); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(perBytes(w), perBytes(want)) {
		t.Errorf("GlobalTWIF-ID = % x, want % x", perBytes(w), perBytes(want))
	}

	if id, bits, ok := g.NodeID(); !ok || id != 0x01020304 || bits != 32 {
		t.Errorf("NodeID() = %x/%d/%v", id, bits, ok)
	}
}

func TestGlobalRANNodeIDRejectsWrongBitLength(t *testing.T) {
	macro := MacroNgENBID(1 << 20)

	for _, tc := range []struct {
		name string
		in   GlobalRANNodeID
	}{
		{"gNB below SIZE(22..32)", NewGlobalGNBID(goldPLMN(), 0, 21)},
		{"gNB above SIZE(22..32)", NewGlobalGNBID(goldPLMN(), 0, 33)},
		{"gNB value wider than its bits", NewGlobalGNBID(goldPLMN(), 1<<22, 22)},
		{"macroNgENB wider than 20 bits", NewGlobalNgENBID(goldPLMN(), NgENBID{MacroNgENBID: &macro})},
		{"no alternative", GlobalRANNodeID{}},
		{"empty gNB-ID", GlobalRANNodeID{GlobalGNBID: &GlobalGNBID{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := tc.in.MarshalPER(w, per.Aligned); err == nil {
				t.Fatalf("marshal accepted %+v", tc.in)
			}
		})
	}
}

func TestGlobalRANNodeIDUnknownChoiceExtensionsArePreserved(t *testing.T) {
	tests := []struct {
		name  string
		build func(*per.Writer) error
		check func(*testing.T, GlobalRANNodeID)
	}{
		{
			"outer GlobalRANNodeID choice-Extensions",
			func(w *per.Writer) error {
				return per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 3, 3)
			},
			func(t *testing.T, g GlobalRANNodeID) {
				t.Helper()

				if g.ChoiceExtension == nil || g.ChoiceExtension.ID != 9999 {
					t.Errorf("ChoiceExtension = %+v", g.ChoiceExtension)
				}
			},
		},
		{
			"nested GNB-ID choice-Extensions",
			func(w *per.Writer) error {
				enc := per.Aligned
				if err := per.EncodeConstrainedWholeNumber(w, enc, 0, 3, 0); err != nil {
					return err
				}

				w.WriteBit(false)
				w.WriteBit(false)

				if err := goldPLMN().MarshalPER(w, enc); err != nil {
					return err
				}

				return per.EncodeConstrainedWholeNumber(w, enc, 0, 1, 1)
			},
			func(t *testing.T, g GlobalRANNodeID) {
				t.Helper()

				if g.GlobalGNBID == nil || g.GlobalGNBID.GNBID.ChoiceExtension == nil {
					t.Errorf("GNB-ID choice-Extensions not kept: %+v", g.GlobalGNBID)
				}
			},
		},
		{
			"nested NgENB-ID choice-Extensions",
			func(w *per.Writer) error {
				enc := per.Aligned
				if err := per.EncodeConstrainedWholeNumber(w, enc, 0, 3, 1); err != nil {
					return err
				}

				w.WriteBit(false)
				w.WriteBit(false)

				if err := goldPLMN().MarshalPER(w, enc); err != nil {
					return err
				}

				return per.EncodeConstrainedWholeNumber(w, enc, 0, 3, 3)
			},
			func(t *testing.T, g GlobalRANNodeID) {
				t.Helper()

				if g.GlobalNgENBID == nil || g.GlobalNgENBID.NgENBID.ChoiceExtension == nil {
					t.Errorf("NgENB-ID choice-Extensions not kept: %+v", g.GlobalNgENBID)
				}
			},
		},
	}

	for _, tt := range tests {
		for _, crit := range []Criticality{CriticalityReject, CriticalityIgnore} {
			t.Run(tt.name+"/"+crit.String(), func(t *testing.T) {
				w := per.NewWriter()
				if err := tt.build(w); err != nil {
					t.Fatal(err)
				}

				if err := encodeContainerField(w, ieField{ID: 9999, Criticality: crit, Value: ap.RawValue([]byte{0x00})}); err != nil {
					t.Fatal(err)
				}

				wire := perBytes(w)

				got, err := unmarshalPERValue[GlobalRANNodeID](wire)
				if err != nil {
					t.Fatalf("decode: %v", err)
				}

				tt.check(t, got)

				if _, _, ok := got.NodeID(); ok {
					t.Error("NodeID() reported an identity for an unknown alternative")
				}

				d := ap.Diagnose[ProtocolIEID](&got)
				if len(d.IEs) != 1 || d.IEs[0].ID != 9999 || d.IEs[0].Criticality != crit {
					t.Errorf("diagnostics = %+v, want the unknown IE on criticality %v", d.IEs, crit)
				}

				w2 := per.NewWriter()
				if err := got.MarshalPER(w2, per.Aligned); err != nil {
					t.Fatal(err)
				}

				if !bytes.Equal(perBytes(w2), wire) {
					t.Errorf("re-encode = % x, want % x", perBytes(w2), wire)
				}
			})
		}
	}
}

func TestTypedChoiceExtensionsKeepWireCriticality(t *testing.T) {
	tngf := TNGFIDValue{BitStringFromUint(0xcafe0001, 32)}
	gTNGF := GlobalTNGFID{PLMNIdentity: goldPLMN(), TNGFID: TNGFID{TNGFID: &tngf}}
	tai := TAI{PLMNIdentity: goldPLMN(), TAC: 9}
	uTWIF := UserLocationInformationTWIF{TWAPID: TWAPID{0x01}, IPAddress: tlaOf(10, 1, 1, 1), TAI: &tai}

	for _, tc := range []struct {
		name  string
		index int64
		ub    int64
		id    ProtocolIEID
		value per.Marshaler
		new   func() interface {
			per.Marshaler
			per.Unmarshaler
		}
		typed func(any) bool
	}{
		{"GlobalTNGF-ID", 3, 3, IDGlobalTNGFID, &gTNGF, func() interface {
			per.Marshaler
			per.Unmarshaler
		} {
			return &GlobalRANNodeID{}
		}, func(v any) bool {
			return v.(*GlobalRANNodeID).GlobalTNGFID != nil && v.(*GlobalRANNodeID).GlobalTNGFID.Equal(gTNGF)
		}},
		{"UserLocationInformationTWIF", 3, 3, IDUserLocationInformationTWIF, &uTWIF, func() interface {
			per.Marshaler
			per.Unmarshaler
		} {
			return &UserLocationInformation{}
		}, func(v any) bool {
			return v.(*UserLocationInformation).TWIF != nil && v.(*UserLocationInformation).TWIF.Equal(uTWIF)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inner, err := per.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}

			w := per.NewWriter()
			if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, tc.ub, tc.index); err != nil {
				t.Fatal(err)
			}

			if err := encodeContainerField(w, ieField{ID: tc.id, Criticality: CriticalityNotify, Value: ap.RawValue(inner)}); err != nil {
				t.Fatal(err)
			}

			wire := perBytes(w)

			v := tc.new()
			if err := v.UnmarshalPER(per.NewReader(wire), per.Aligned); err != nil {
				t.Fatal(err)
			}

			if !tc.typed(v) {
				t.Fatalf("decoded %+v, want the typed alternative", v)
			}

			re := per.NewWriter()
			if err := v.MarshalPER(re, per.Aligned); err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(perBytes(re), wire) {
				t.Errorf("re-encode = % x, want % x", perBytes(re), wire)
			}
		})
	}
}
