// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package impl

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

	*o = octet(v)

	return nil
}

func (o *octet) Diagnose(*Diagnosis[uint16]) bool { return *o == notComprehendedOctet }

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
	Clear: func(m *item) { m.Ext = nil },
}}

func (it *item) Diagnose(d *Diagnosis[uint16]) bool {
	DiagnoseExtensions(d, itemExtIEs, it, &it.ext)

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
	if err := DecodeMessage(testProfile, msgIEs, b, m, &m.meta, false); err != nil {
		return nil, err
	}

	return m, nil
}

func parseMsg(b []byte) (*msg, error) {
	m := new(msg)
	if err := DecodeMessage(testProfile, msgIEs, b, m, &m.meta, true); err != nil {
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
		{"value not comprehended", []Field[uint16]{valueReject}, []Kind{KindNotComprehended}, &wantCheck{ies: []uint16{idA}}},
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

func TestDecodeKeepsNotComprehendedValuesAndParsePrunesThem(t *testing.T) {
	value := messageValue(t,
		Field[uint16]{ID: idA, Criticality: CriticalityIgnore, Value: encodeValue(t, octet(notComprehendedOctet))},
		Field[uint16]{ID: idB, Criticality: CriticalityIgnore, Value: encodeValue(t, &item{X: 4, Ext: ptr(octet(notComprehendedOctet))})},
	)

	decoded, err := decodeMsg(value)
	if err != nil {
		t.Fatal(err)
	}

	if decoded.A == nil || *decoded.A != notComprehendedOctet || decoded.B == nil || decoded.B.Ext == nil || *decoded.B.Ext != notComprehendedOctet {
		t.Fatalf("decode dropped a not comprehended value: A %v B %+v", decoded.A, decoded.B)
	}

	want := []Diagnostic[uint16]{
		{ID: idA, Criticality: CriticalityIgnore, Kind: KindNotComprehended},
		{Path: []uint16{idB}, ID: idItem, Criticality: CriticalityIgnore, Kind: KindNotComprehended},
	}

	got := decoded.meta.Diagnostics().IEs
	if len(got) != len(want) {
		t.Fatalf("diagnostics = %v, want %v", got, want)
	}

	for i := range want {
		if got[i].ID != want[i].ID || got[i].Kind != want[i].Kind || !slices.Equal(got[i].Path, want[i].Path) {
			t.Fatalf("diagnostics = %v, want %v", got, want)
		}
	}

	parsed, err := parseMsg(value)
	if err != nil {
		t.Fatal(err)
	}

	if parsed.A != nil || parsed.B == nil || parsed.B.X != 4 || parsed.B.Ext != nil {
		t.Fatalf("parse kept a not comprehended IE: A %v B %+v", parsed.A, parsed.B)
	}

	if !slices.EqualFunc(parsed.meta.Diagnostics().IEs, got, func(a, b Diagnostic[uint16]) bool {
		return a.ID == b.ID && a.Kind == b.Kind && slices.Equal(a.Path, b.Path)
	}) {
		t.Fatalf("parse diagnostics %v differ from decode diagnostics %v", parsed.meta.Diagnostics().IEs, got)
	}

	for name, m := range map[string]*msg{"decoded": decoded, "parsed": parsed} {
		if b := encodeMsg(t, m); !bytes.Equal(b, value) {
			t.Fatalf("%s re-encoded %x, want %x", name, b, value)
		}
	}
}

func TestPrunedExtensionStaysInWireOrder(t *testing.T) {
	ext := func(id uint16, v octet) Field[uint16] {
		return Field[uint16]{ID: id, Criticality: CriticalityIgnore, Value: encodeValue(t, v)}
	}

	b := itemValue(t, ext(600, 1), ext(idItem, notComprehendedOctet), ext(601, 2))

	var it item
	if err := DecodeValue(b, &it); err != nil {
		t.Fatal(err)
	}

	Prune[uint16](&it)

	if it.Ext != nil {
		t.Fatalf("Ext = %v, want pruned", *it.Ext)
	}

	if got := encodeValue(t, &it); !bytes.Equal(got, b) {
		t.Fatalf("re-encoded %x, want %x", got, b)
	}

	d := Diagnose[uint16](&it)
	for _, e := range d.IEs {
		if e.Kind == KindMissing {
			t.Fatalf("a pruned extension is reported missing: %v", d.IEs)
		}
	}
}

func TestDecodeValueRejectsTrailingOctets(t *testing.T) {
	b := append(itemValue(t), 0x00)

	var it item
	if err := DecodeValue(b, &it); !errors.Is(err, per.ErrTrailing) {
		t.Fatalf("DecodeValue = %v, want per.ErrTrailing", err)
	}

	m := messageValue(t, Field[uint16]{ID: idA, Criticality: CriticalityReject, Value: append(encodeValue(t, octet(1)), 0x00)})
	if _, err := decodeMsg(m); !errors.Is(err, per.ErrTrailing) {
		t.Fatalf("decode with a trailing octet in an IE value = %v, want per.ErrTrailing", err)
	}

	if _, err := decodeMsg(append(messageValue(t, Field[uint16]{ID: idA, Criticality: CriticalityReject, Value: []byte{0x01}}), 0x00)); !errors.Is(err, per.ErrTrailing) {
		t.Fatalf("decode with a trailing octet after the message = %v, want per.ErrTrailing", err)
	}
}

type choice struct {
	Known *octet
	Raw   *SingleContainer[uint16]
}

func (c *choice) Diagnose(d *Diagnosis[uint16]) bool {
	switch {
	case c.Known != nil:
		return DiagnoseChoiceExtension(d, idItem, CriticalityIgnore, &c.Known, &c.Raw)
	case c.Raw != nil:
		return DiagnoseUnknownChoiceExtension(d, c.Raw)
	}

	return false
}

func TestChoiceExtensionNotComprehendedReachesTheEnclosingIE(t *testing.T) {
	for name, c := range map[string]*choice{
		"unknown alternative":       {Raw: &SingleContainer[uint16]{ID: 999, Criticality: CriticalityIgnore, Value: "\x00"}},
		"not comprehended typed IE": {Known: ptr(octet(notComprehendedOctet))},
	} {
		var d Diagnosis[uint16]

		if !d.IE(idB, CriticalityReject, c) {
			t.Errorf("%s: the enclosing IE is comprehended", name)
		}

		var ase *AbstractSyntaxError[uint16]
		if err := d.Diagnostics().Check(); !errors.As(err, &ase) || len(ase.IEs) != 1 || ase.IEs[0].ID != idB {
			t.Errorf("%s: Check() = %v, want a reject on the enclosing IE", name, err)
		}
	}

	c := &choice{Known: ptr(octet(notComprehendedOctet))}
	Prune[uint16](c)

	if c.Known != nil || c.Raw == nil || c.Raw.ID != idItem || c.Raw.Value != string(encodeValue(t, octet(notComprehendedOctet))) {
		t.Fatalf("pruned choice = %+v", c)
	}
}

func ptr[T any](v T) *T { return &v }
