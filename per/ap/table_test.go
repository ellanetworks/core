// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ap

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/ellanetworks/core/per"
)

type octet uint8

const notComprehendedOctet = 0xff

func (o octet) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeConstrainedWholeNumber(w, enc, 0, 255, int64(o))
}

func (o *octet) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeConstrainedWholeNumber(r, enc, 0, 255)
	if err != nil {
		return err
	}

	if v == notComprehendedOctet {
		return ErrNotComprehended
	}

	*o = octet(v)

	return nil
}

type item struct {
	X   octet
	Ext *octet
	ext Preserved[uint16]
}

const (
	idA    = 1
	idB    = 2
	idC    = 3
	idItem = 100
)

var itemExtIEs = []IESpec[uint16, item]{{
	ID: idItem, Presence: PresenceOptional, Criticality: CriticalityIgnore,
	Decode: func(m *item, r *per.Reader, enc per.Encoding) error {
		var v octet
		if err := v.UnmarshalPER(r, enc); err != nil {
			return err
		}

		m.Ext = &v

		return nil
	},
	Encode: func(m *item) (per.Marshaler, bool) {
		if m.Ext == nil {
			return nil, false
		}

		return m.Ext, true
	},
}}

func (it *item) Diagnose(d *Diagnosis[uint16]) bool {
	DiagnoseExtensions(d, itemExtIEs, it, it.ext)

	return it.X == unknownOctet
}

const unknownOctet = 0xfe

func (it *item) MarshalPER(w *per.Writer, enc per.Encoding) error {
	ext := ExtensionFields(itemExtIEs, it, it.ext)
	w.WriteBit(len(ext) > 0)

	if err := it.X.MarshalPER(w, enc); err != nil {
		return err
	}

	if len(ext) > 0 {
		return EncodeExtensionContainer(testProfile, w, enc, ext)
	}

	return nil
}

func (it *item) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	present, err := r.ReadBit()
	if err != nil {
		return err
	}

	if err := it.X.UnmarshalPER(r, enc); err != nil {
		return err
	}

	if present {
		return DecodeExtensionContainer(testProfile, r, enc, itemExtIEs, it, &it.ext)
	}

	return nil
}

type msg struct {
	A    *octet
	B    *item
	C    *octet
	meta Message[uint16]
}

func optional[T any, PT interface {
	*T
	per.Marshaler
	per.Unmarshaler
}](id uint16, presence Presence, crit Criticality, field func(*msg) **T) IESpec[uint16, msg] {
	return IESpec[uint16, msg]{
		ID: id, Presence: presence, Criticality: crit,
		Decode: func(m *msg, r *per.Reader, enc per.Encoding) error {
			v := PT(new(T))
			if err := v.UnmarshalPER(r, enc); err != nil {
				return err
			}

			*field(m) = (*T)(v)

			return nil
		},
		Encode: func(m *msg) (per.Marshaler, bool) {
			if *field(m) == nil {
				return nil, false
			}

			return PT(*field(m)), true
		},
	}
}

var msgIEs = func() []IESpec[uint16, msg] {
	c := optional[octet](idC, PresenceConditional, CriticalityReject, func(m *msg) **octet { return &m.C })
	c.Condition = func(m *msg) bool { return m.A != nil && *m.A == 1 }

	return []IESpec[uint16, msg]{
		optional[octet](idA, PresenceMandatory, CriticalityReject, func(m *msg) **octet { return &m.A }),
		optional[item](idB, PresenceOptional, CriticalityIgnore, func(m *msg) **item { return &m.B }),
		c,
	}
}()

func encodeValue(t testing.TB, m per.Marshaler) []byte {
	t.Helper()

	b, err := per.Marshal(m, per.Aligned)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func itemValue(t testing.TB, ext ...Field[uint16]) []byte {
	t.Helper()

	w := per.NewWriter()
	w.WriteBit(len(ext) > 0)

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 255, 7); err != nil {
		t.Fatal(err)
	}

	if len(ext) > 0 {
		fv := make([]FieldValue[uint16], len(ext))
		for i, f := range ext {
			fv[i] = f.Encodable()
		}

		if err := EncodeExtensionContainer(testProfile, w, per.Aligned, fv); err != nil {
			t.Fatal(err)
		}
	}

	w.AlignToByte()

	return w.Bytes()
}

func messageValue(t testing.TB, fields ...Field[uint16]) []byte {
	t.Helper()

	w := per.NewWriter()
	w.WriteBit(false)

	fv := make([]FieldValue[uint16], len(fields))
	for i, f := range fields {
		fv[i] = f.Encodable()
	}

	if err := EncodeIEContainer(testProfile, w, per.Aligned, fv); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	return w.Bytes()
}

func decodeMsg(b []byte) (*msg, error) {
	m := new(msg)
	if err := DecodeMessage(testProfile, msgIEs, b, m, &m.meta); err != nil {
		return nil, err
	}

	return m, nil
}

func encodeMsg(t testing.TB, m *msg) []byte {
	t.Helper()

	w := per.NewWriter()
	if err := EncodeMessage(testProfile, w, per.Aligned, msgIEs, m, &m.meta); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	return w.Bytes()
}

type wantCheck struct {
	falsely bool
	ies     []uint16
}

func TestDecodeIsTotalForAbstractSyntaxErrors(t *testing.T) {
	one := Field[uint16]{ID: idA, Criticality: CriticalityReject, Value: encodeValue(t, octet(1))}
	two := Field[uint16]{ID: idA, Criticality: CriticalityReject, Value: encodeValue(t, octet(2))}
	b := Field[uint16]{ID: idB, Criticality: CriticalityIgnore, Value: itemValue(t)}
	c := Field[uint16]{ID: idC, Criticality: CriticalityReject, Value: encodeValue(t, octet(9))}
	unknownIgnore := Field[uint16]{ID: 500, Criticality: CriticalityIgnore, Value: []byte{0xab}}
	unknownReject := Field[uint16]{ID: 501, Criticality: CriticalityReject, Value: []byte{0xcd}}
	nestedReject := Field[uint16]{ID: idB, Criticality: CriticalityIgnore, Value: itemValue(t, Field[uint16]{ID: 502, Criticality: CriticalityReject, Value: []byte{0x01}})}
	valueReject := Field[uint16]{ID: idA, Criticality: CriticalityReject, Value: encodeValue(t, octet(notComprehendedOctet))}
	unknownItem := Field[uint16]{ID: idB, Criticality: CriticalityReject, Value: encodeValue(t, &item{X: unknownOctet})}

	cases := []struct {
		name   string
		fields []Field[uint16]
		kinds  []Kind
		check  *wantCheck
	}{
		{"well formed", []Field[uint16]{one, b, c}, nil, nil},
		{"unknown ignore IE", []Field[uint16]{one, unknownIgnore, c}, []Kind{KindNotComprehended}, nil},
		{"unknown reject IE", []Field[uint16]{one, unknownReject, c}, []Kind{KindNotComprehended}, &wantCheck{ies: []uint16{501}}},
		{"unknown reject extension nested in an item", []Field[uint16]{two, nestedReject}, []Kind{KindNotComprehended}, &wantCheck{ies: []uint16{502}}},
		{"value not comprehended", []Field[uint16]{valueReject}, []Kind{KindNotComprehended, KindMissing}, &wantCheck{ies: []uint16{idA}}},
		{"value decoded but not comprehended", []Field[uint16]{two, unknownItem}, []Kind{KindNotComprehended}, &wantCheck{ies: []uint16{idB}}},
		{"missing mandatory IE", []Field[uint16]{b}, []Kind{KindMissing}, &wantCheck{ies: []uint16{idA}}},
		{"duplicate IE", []Field[uint16]{two, two}, []Kind{KindTooManyOccurrences}, &wantCheck{falsely: true}},
		{"wrong order", []Field[uint16]{b, two}, []Kind{KindWrongOrder}, &wantCheck{falsely: true}},
		{"erroneously present conditional IE", []Field[uint16]{two, c}, []Kind{KindErroneouslyPresent}, &wantCheck{falsely: true}},
		{"reject before wrong order", []Field[uint16]{unknownReject, b, two, unknownReject}, []Kind{KindNotComprehended, KindWrongOrder, KindNotComprehended}, &wantCheck{falsely: true, ies: []uint16{501}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value := messageValue(t, tc.fields...)

			m, err := decodeMsg(value)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			d := m.meta.Diagnostics()

			kinds := make([]Kind, len(d.IEs))
			for i, e := range d.IEs {
				kinds[i] = e.Kind
			}

			if !slices.Equal(kinds, tc.kinds) {
				t.Errorf("kinds = %v, want %v", kinds, tc.kinds)
			}

			if got := encodeMsg(t, m); !bytes.Equal(got, value) {
				t.Fatalf("re-encoded\n got  %x\n want %x", got, value)
			}

			err = d.Check()
			if tc.check == nil {
				if err != nil {
					t.Fatalf("Check() = %v, want nil", err)
				}

				return
			}

			var ase *AbstractSyntaxError[uint16]
			if !errors.As(err, &ase) {
				t.Fatalf("Check() = %v, want *AbstractSyntaxError", err)
			}

			ids := make([]uint16, len(ase.IEs))
			for i, e := range ase.IEs {
				ids[i] = e.ID
			}

			if ase.FalselyConstructed != tc.check.falsely || !slices.Equal(ids, tc.check.ies) {
				t.Fatalf("Check() = falsely %v ids %v, want falsely %v ids %v", ase.FalselyConstructed, ids, tc.check.falsely, tc.check.ies)
			}
		})
	}
}

func TestNestedDiagnosticCarriesPath(t *testing.T) {
	value := messageValue(t,
		Field[uint16]{ID: idA, Criticality: CriticalityReject, Value: encodeValue(t, octet(2))},
		Field[uint16]{ID: idB, Criticality: CriticalityIgnore, Value: itemValue(t, Field[uint16]{ID: 502, Criticality: CriticalityNotify, Value: []byte{0x01}})},
	)

	m, err := decodeMsg(value)
	if err != nil {
		t.Fatal(err)
	}

	d := m.meta.Diagnostics()
	if len(d.IEs) != 1 || !slices.Equal(d.IEs[0].Path, []uint16{idB}) || d.IEs[0].ID != 502 {
		t.Fatalf("diagnostics = %+v", d.IEs)
	}

	if !d.ReportRequired() || d.Check() != nil {
		t.Fatalf("ReportRequired = %v, Check = %v", d.ReportRequired(), d.Check())
	}

	if got := m.B.ext.UnknownIEs(); len(got) != 1 || got[0].ID != 502 {
		t.Fatalf("item preserved %+v", got)
	}
}

func TestExtensionContainerDecodeIsLenient(t *testing.T) {
	for _, c := range []struct {
		crit   Criticality
		reject bool
	}{{CriticalityReject, true}, {CriticalityIgnore, false}} {
		var it item

		b := itemValue(t, Field[uint16]{ID: 502, Criticality: c.crit, Value: []byte{0x01}})
		if err := it.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
			t.Fatalf("%v: decode is lenient, got %v", c.crit, err)
		}

		if got := encodeValue(t, &it); !bytes.Equal(got, b) {
			t.Fatalf("%v: re-encoded %x, want %x", c.crit, got, b)
		}

		d := Diagnose[uint16](&it)
		if len(d.IEs) != 1 || d.IEs[0].ID != 502 || d.IEs[0].Criticality != c.crit || len(d.IEs[0].Path) != 0 {
			t.Fatalf("%v: diagnostics = %+v", c.crit, d.IEs)
		}

		if (d.Check() != nil) != c.reject {
			t.Fatalf("%v: Check() = %v", c.crit, d.Check())
		}
	}
}

func TestExtensionContainerDiagnosesOrderAndDuplicates(t *testing.T) {
	ext := Field[uint16]{ID: idItem, Criticality: CriticalityIgnore, Value: encodeValue(t, octet(3))}

	var it item

	b := itemValue(t, ext, ext)
	if err := it.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		t.Fatal(err)
	}

	if got := encodeValue(t, &it); !bytes.Equal(got, b) {
		t.Fatalf("re-encoded %x, want %x", got, b)
	}

	d := Diagnose[uint16](&it)
	if len(d.IEs) != 1 || d.IEs[0].Kind != KindTooManyOccurrences {
		t.Fatalf("diagnostics = %+v, want one duplicate", d.IEs)
	}

	var ase *AbstractSyntaxError[uint16]
	if !errors.As(d.Check(), &ase) || !ase.FalselyConstructed {
		t.Fatalf("Check() = %v, want a falsely constructed message", d.Check())
	}
}

func TestDiagnoseReportsRootNotComprehended(t *testing.T) {
	it := item{X: unknownOctet}

	d := Diagnose[uint16](&it)
	if !d.RootNotComprehended || d.Empty() || d.Check() == nil {
		t.Fatalf("diagnostics = %+v, Check = %v", d, d.Check())
	}
}

func TestPreservedIsComparable(t *testing.T) {
	var a, b Preserved[uint16]

	a.Append(Field[uint16]{ID: 1, Criticality: CriticalityIgnore, Value: []byte{0x01}})
	b.Append(Field[uint16]{ID: 1, Criticality: CriticalityIgnore, Value: []byte{0x01}})

	if a != b {
		t.Fatal("equal preserved state compared unequal")
	}

	set := map[item]bool{{X: 1}: true}
	if !set[item{X: 1}] {
		t.Fatal("item not usable as a map key")
	}
}

func FuzzDecodeMessage(f *testing.F) {
	f.Add(messageValue(f, Field[uint16]{ID: idA, Criticality: CriticalityReject, Value: []byte{0x01}}))
	f.Add(messageValue(f,
		Field[uint16]{ID: idB, Criticality: CriticalityIgnore, Value: itemValue(f, Field[uint16]{ID: 502, Criticality: CriticalityReject, Value: []byte{0x01}})},
		Field[uint16]{ID: idA, Criticality: CriticalityNotify, Value: []byte{0x02}},
		Field[uint16]{ID: idA, Criticality: CriticalityReject, Value: []byte{0xff}},
		Field[uint16]{ID: 9, Criticality: CriticalityIgnore, Value: []byte{0x00}},
	))

	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := decodeMsg(data)
		if err != nil {
			return
		}

		w := per.NewWriter()
		if err := EncodeMessage(testProfile, w, per.Aligned, msgIEs, m, &m.meta); err != nil {
			t.Fatalf("encode: %v", err)
		}

		w.AlignToByte()

		again, err := decodeMsg(w.Bytes())
		if err != nil {
			t.Fatalf("re-decode: %v", err)
		}

		w2 := per.NewWriter()
		if err := EncodeMessage(testProfile, w2, per.Aligned, msgIEs, again, &again.meta); err != nil {
			t.Fatalf("re-encode: %v", err)
		}

		w2.AlignToByte()

		if !bytes.Equal(w.Bytes(), w2.Bytes()) {
			t.Fatalf("not a fixed point:\n%x\n%x", w.Bytes(), w2.Bytes())
		}
	})
}
