// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package protocolie

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

func encodeBody(t testing.TB, fields []Field) []byte {
	t.Helper()

	w := per.NewWriter()
	w.WriteBit(false)

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(len(fields))); err != nil {
		t.Fatal(err)
	}

	for _, f := range fields {
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(f.ID)); err != nil {
			t.Fatal(err)
		}

		if err := per.EncodeEnumerated(w, per.Aligned, criticalityCount, false, int64(f.Criticality)); err != nil {
			t.Fatal(err)
		}

		if err := per.EncodeOpenTypeBytes(w, per.Aligned, f.Value); err != nil {
			t.Fatal(err)
		}
	}

	return w.Bytes()
}

func encodePDU(t testing.TB, choice int, pc uint8, crit uint8, body []byte) []byte {
	t.Helper()

	w := per.NewWriter()
	w.WriteBit(false)

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, pduChoiceCount-1, int64(choice)); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProcedureCode, int64(pc)); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeEnumerated(w, per.Aligned, criticalityCount, false, int64(crit)); err != nil {
		t.Fatal(err)
	}

	if err := per.EncodeOpenTypeBytes(w, per.Aligned, body); err != nil {
		t.Fatal(err)
	}

	return w.Bytes()
}

func mustParse(t testing.TB, b []byte) *Message {
	t.Helper()

	m, err := Parse(b, nil)
	if err != nil {
		t.Fatalf("Parse(% x): %v", b, err)
	}

	return m
}

func mustBytes(t testing.TB, m *Message) []byte {
	t.Helper()

	b, err := m.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func TestParseHeaderAndFields(t *testing.T) {
	fields := []Field{
		{ID: 0, Criticality: 0, Value: []byte{0x00, 0x01}},
		{ID: 8, Criticality: 0, Value: []byte{0x00, 0x07}},
		{ID: 26, Criticality: 1, Value: []byte{0x01, 0x07}},
	}
	b := encodePDU(t, 1, 11, 1, encodeBody(t, fields))

	m := mustParse(t, b)

	if m.Choice() != 1 || m.ProcedureCode() != 11 || m.Criticality() != 1 {
		t.Fatalf("header = %d/%d/%d", m.Choice(), m.ProcedureCode(), m.Criticality())
	}

	got := m.Fields()
	if len(got) != len(fields) {
		t.Fatalf("got %d fields, want %d", len(got), len(fields))
	}

	for i := range fields {
		if got[i].ID != fields[i].ID || got[i].Criticality != fields[i].Criticality || !bytes.Equal(got[i].Value, fields[i].Value) {
			t.Errorf("field %d = %+v, want %+v", i, got[i], fields[i])
		}
	}

	if m.Index(26) != 2 || m.Index(99) != -1 {
		t.Errorf("Index: %d %d", m.Index(26), m.Index(99))
	}

	if !bytes.Equal(mustBytes(t, m), b) {
		t.Fatal("unmodified re-emit differs")
	}
}

func TestParseIndependentOfInput(t *testing.T) {
	fields := []Field{
		{ID: 0, Criticality: 0, Value: []byte{0x00, 0x01}},
		{ID: 8, Criticality: 0, Value: []byte{0x00, 0x07}},
		{ID: 26, Criticality: 1, Value: []byte{0x01, 0x07}},
	}
	b := encodePDU(t, 1, 11, 1, encodeBody(t, fields))
	orig := bytes.Clone(b)

	unmodified := mustParse(t, b)
	modified := mustParse(t, b)

	if err := modified.SetValue(1, []byte{0x00, 0x09}); err != nil {
		t.Fatal(err)
	}

	for i := range b {
		b[i] = 0xff
	}

	if got := mustBytes(t, unmodified); !bytes.Equal(got, orig) {
		t.Fatalf("unmodified re-emit = % x, want % x", got, orig)
	}

	fields[1].Value = []byte{0x00, 0x09}

	want := encodePDU(t, 1, 11, 1, encodeBody(t, fields))
	if got := mustBytes(t, modified); !bytes.Equal(got, want) {
		t.Fatalf("modified re-emit = % x, want % x", got, want)
	}
}

func TestEmptyContainer(t *testing.T) {
	b := encodePDU(t, 0, 14, 0, encodeBody(t, nil))
	m := mustParse(t, b)

	if m.Len() != 0 {
		t.Fatalf("Len = %d", m.Len())
	}

	if err := m.Insert(0, Field{ID: 2, Criticality: 1, Value: []byte{0x00}}); err != nil {
		t.Fatal(err)
	}

	want := encodePDU(t, 0, 14, 0, encodeBody(t, []Field{{ID: 2, Criticality: 1, Value: []byte{0x00}}}))
	if got := mustBytes(t, m); !bytes.Equal(got, want) {
		t.Fatalf("got % x, want % x", got, want)
	}
}

func TestSetValueLengthBoundaries(t *testing.T) {
	sizes := []int{1, 2, 127, 128, 16383, 16384, 16385, 32768, 65536, 65537, 70000}

	for _, from := range sizes {
		for _, to := range sizes {
			fields := []Field{
				{ID: 1, Criticality: 0, Value: bytes.Repeat([]byte{0xa1}, 5)},
				{ID: 2, Criticality: 1, Value: bytes.Repeat([]byte{0xb2}, from)},
				{ID: 3, Criticality: 2, Value: bytes.Repeat([]byte{0xc3}, 3)},
			}
			b := encodePDU(t, 0, 9, 0, encodeBody(t, fields))
			m := mustParse(t, b)

			if !bytes.Equal(mustBytes(t, m), b) {
				t.Fatalf("%d: identity failed", from)
			}

			nv := bytes.Repeat([]byte{0xd4}, to)
			if err := m.SetValue(1, nv); err != nil {
				t.Fatal(err)
			}

			fields[1].Value = nv
			want := encodePDU(t, 0, 9, 0, encodeBody(t, fields))

			if got := mustBytes(t, m); !bytes.Equal(got, want) {
				t.Fatalf("%d -> %d: re-encoded PDU differs from a fresh encoding", from, to)
			}

			back := mustParse(t, want)
			if f, _ := back.Field(1); !bytes.Equal(f.Value, nv) {
				t.Fatalf("%d -> %d: value did not survive", from, to)
			}
		}
	}
}

func TestSetSameValueIsIdentity(t *testing.T) {
	fields := []Field{
		{ID: 1, Criticality: 0, Value: bytes.Repeat([]byte{0x11}, 40000)},
		{ID: 2, Criticality: 1, Value: []byte{0x22}},
	}
	b := encodePDU(t, 2, 3, 0, encodeBody(t, fields))
	m := mustParse(t, b)

	for i := range m.Len() {
		f, _ := m.Field(i)
		if err := m.SetValue(i, f.Value); err != nil {
			t.Fatal(err)
		}
	}

	if !bytes.Equal(mustBytes(t, m), b) {
		t.Fatal("replacing every value with itself changed the PDU")
	}
}

func TestNonCanonicalDeterminantKeptWhenUntouched(t *testing.T) {
	b := []byte{
		0x00, 0x0b, 0x40, 0x10,
		0x00, 0x00, 0x02,
		0x00, 0x00, 0x00, 0x80, 0x02, 0x00, 0x01,
		0x00, 0x08, 0x00, 0x02, 0x00, 0x05,
	}
	m := mustParse(t, b)

	if !bytes.Equal(mustBytes(t, m), b) {
		t.Fatal("identity failed")
	}

	if err := m.SetValue(1, []byte{0x00, 0x09}); err != nil {
		t.Fatal(err)
	}

	got := mustBytes(t, m)

	want := append([]byte(nil), b...)
	want[len(want)-1] = 0x09

	if !bytes.Equal(got, want) {
		t.Fatalf("got % x, want % x", got, want)
	}

	if err := m.SetValue(0, []byte{0x00, 0x03}); err != nil {
		t.Fatal(err)
	}

	got = mustBytes(t, m)
	want[13] = 0x03

	if !bytes.Equal(got, want) {
		t.Fatalf("same-length replace must keep the sender's determinant: got % x, want % x", got, want)
	}
}

func TestPaddingTailAndExtensionKept(t *testing.T) {
	body := encodeBody(t, []Field{{ID: 5, Criticality: 0, Value: []byte{0x01}}})
	body[0] |= 0x80
	body = append(body, 0x01, 0x80, 0x01, 0xff)

	b := encodePDU(t, 0, 7, 0, body)
	b[0] |= 0x01
	b = append(b, 0xee)

	m := mustParse(t, b)

	if err := m.SetValue(0, []byte{0x02, 0x03}); err != nil {
		t.Fatal(err)
	}

	got := mustBytes(t, m)

	if got[0] != b[0] || got[len(got)-1] != 0xee {
		t.Fatalf("header padding or PDU tail lost: % x", got)
	}

	back := mustParse(t, got)
	if !bytes.Equal(back.bodyTail, []byte{0x01, 0x80, 0x01, 0xff}) || back.bodyHdr[0]&0x80 == 0 {
		t.Fatalf("message extension lost: hdr % x tail % x", back.bodyHdr, back.bodyTail)
	}
}

func TestInsertRemove(t *testing.T) {
	fields := []Field{
		{ID: 1, Criticality: 0, Value: []byte{0x01}},
		{ID: 2, Criticality: 1, Value: []byte{0x02}},
	}
	b := encodePDU(t, 0, 1, 0, encodeBody(t, fields))
	m := mustParse(t, b)

	if err := m.Insert(1, Field{ID: 9, Criticality: 2, Value: []byte{0x09, 0x09}}); err != nil {
		t.Fatal(err)
	}

	if err := m.Remove(0); err != nil {
		t.Fatal(err)
	}

	want := encodePDU(t, 0, 1, 0, encodeBody(t, []Field{{ID: 9, Criticality: 2, Value: []byte{0x09, 0x09}}, fields[1]}))
	if got := mustBytes(t, m); !bytes.Equal(got, want) {
		t.Fatalf("got % x, want % x", got, want)
	}

	if err := m.Insert(5, Field{}); !errors.Is(err, ErrIndex) {
		t.Errorf("Insert past end: %v", err)
	}

	if err := m.Insert(0, Field{Criticality: 3}); !errors.Is(err, ErrCriticality) {
		t.Errorf("Insert criticality 3: %v", err)
	}

	if err := m.Remove(-1); !errors.Is(err, ErrIndex) {
		t.Errorf("Remove(-1): %v", err)
	}

	if err := m.SetValue(2, nil); !errors.Is(err, ErrIndex) {
		t.Errorf("SetValue(2): %v", err)
	}
}

func TestRefusals(t *testing.T) {
	body := encodeBody(t, nil)

	if _, err := Parse(encodePDU(t, 0, 39, 1, body), func(pc uint8) bool { return pc == 39 }); !errors.Is(err, ErrNotProtocolIEContainer) {
		t.Errorf("private message: %v", err)
	}

	ext := encodePDU(t, 0, 1, 0, body)
	ext[0] |= 0x80

	if _, err := Parse(ext, nil); !errors.Is(err, ErrExtensionAlternative) {
		t.Errorf("extension alternative: %v", err)
	}

	bad := encodePDU(t, 0, 1, 0, body)
	bad[0] |= 0x60

	if _, err := Parse(bad, nil); err == nil {
		t.Error("choice index 3 accepted")
	}

	badCrit := encodePDU(t, 0, 1, 0, encodeBody(t, []Field{{ID: 1, Value: []byte{0x00}}}))
	badCrit[9] = 0xc0

	if _, err := Parse(badCrit, nil); err == nil {
		t.Error("IE criticality 3 accepted")
	}

	inflated := encodePDU(t, 0, 1, 0, []byte{0x00, 0x00, 0x05, 0x00, 0x01, 0x00, 0x01, 0x00})
	if _, err := Parse(inflated, nil); err == nil {
		t.Error("count larger than the content accepted")
	}

	full := encodePDU(t, 0, 1, 0, encodeBody(t, []Field{{ID: 1, Value: []byte{0x00}}}))
	for i := range len(full) - 1 {
		if _, err := Parse(full[:i], nil); err == nil {
			t.Errorf("truncation at %d accepted", i)
		}
	}
}

func TestFragmentedOuterBody(t *testing.T) {
	fields := []Field{
		{ID: 1, Criticality: 0, Value: bytes.Repeat([]byte{0x01}, 20000)},
		{ID: 2, Criticality: 0, Value: []byte{0x00, 0x05}},
	}
	b := encodePDU(t, 0, 0, 0, encodeBody(t, fields))
	m := mustParse(t, b)

	if m.body.det != nil {
		t.Fatal("outer open type should be fragmented")
	}

	if err := m.SetValue(1, []byte{0x40, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}

	fields[1].Value = []byte{0x40, 0x01, 0x00}
	want := encodePDU(t, 0, 0, 0, encodeBody(t, fields))

	if got := mustBytes(t, m); !bytes.Equal(got, want) {
		t.Fatal("fragmented body not re-encoded canonically")
	}
}

func FuzzParse(f *testing.F) {
	f.Add(encodePDU(f, 0, 12, 1, encodeBody(f, []Field{{ID: 8, Value: []byte{0x00, 0x01}}, {ID: 26, Criticality: 1, Value: []byte{0x01, 0x07}}})))
	f.Add(encodePDU(f, 1, 14, 0, encodeBody(f, nil)))
	f.Add(encodePDU(f, 2, 17, 0, encodeBody(f, []Field{{ID: 2, Criticality: 1, Value: bytes.Repeat([]byte{0x33}, 300)}})))
	f.Add([]byte{0x30, 0x30, 0x30, 0x0a, 0x30, 0x00, 0x01, 0x30, 0x30, 0x30, 0x00, 0x00, 0x00, 0x00})
	f.Add([]byte{0x00, 0x0b, 0x40, 0x10, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x80, 0x02, 0x00, 0x01, 0x00, 0x08, 0x00, 0x02, 0x00, 0x05})

	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := Parse(data, nil)
		if err != nil {
			return
		}

		out, err := m.Bytes()
		if err != nil {
			t.Fatalf("Bytes: %v", err)
		}

		if !bytes.Equal(out, data) {
			t.Fatalf("unmodified re-emit differs")
		}

		for i := range m.Len() {
			fld, err := m.Field(i)
			if err != nil {
				t.Fatal(err)
			}

			if err := m.SetValue(i, fld.Value); err != nil {
				t.Fatal(err)
			}
		}

		out, err = m.Bytes()
		if err != nil {
			t.Fatalf("Bytes after same-value replace: %v", err)
		}

		if !bytes.Equal(out, data) {
			t.Fatalf("same-value replace is not identity")
		}

		if m.Len() == 0 {
			return
		}

		if err := m.SetValue(0, []byte{0x5a, 0x5a, 0x5a}); err != nil {
			t.Fatal(err)
		}

		out, err = m.Bytes()
		if err != nil {
			t.Fatalf("Bytes after replace: %v", err)
		}

		back, err := Parse(out, nil)
		if err != nil {
			t.Fatalf("re-parse after replace: %v", err)
		}

		if back.Len() != m.Len() {
			t.Fatalf("field count changed: %d -> %d", m.Len(), back.Len())
		}

		if fld, _ := back.Field(0); !bytes.Equal(fld.Value, []byte{0x5a, 0x5a, 0x5a}) {
			t.Fatalf("replaced value lost: % x", fld.Value)
		}
	})
}
