// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package impl

import (
	"bytes"
	"slices"
	"testing"

	"github.com/ellanetworks/core/per"
)

const (
	testItemID    uint16 = 7
	testForeignID uint16 = 900
	testListUB           = 16
)

type testItem struct {
	V uint8
	ListItem[uint16]
}

func (x *testItem) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeConstrainedWholeNumber(w, enc, 0, 255, int64(x.V))
}

func (x *testItem) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeConstrainedWholeNumber(r, enc, 0, 255)
	x.V = uint8(v)

	return err
}

func (x *testItem) Diagnose(*Diagnosis[uint16]) bool { return x.V == notComprehendedOctet }

func typedField(v uint8) FieldValue[uint16] {
	return FieldValue[uint16]{ID: testItemID, Criticality: CriticalityIgnore, Value: &testItem{V: v}}
}

func foreignField(crit Criticality) FieldValue[uint16] {
	return FieldValue[uint16]{ID: testForeignID, Criticality: crit, Value: RawValue{0xab, 0xcd}}
}

func encodeTestList(t *testing.T, fields []FieldValue[uint16]) []byte {
	t.Helper()

	w := per.NewWriter()

	err := per.EncodeLength(w, per.Aligned, 1, testListUB, true, int64(len(fields)), func(int64) error {
		for _, f := range fields {
			if err := EncodeField(testProfile, w, per.Aligned, f); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	return w.Bytes()
}

func TestItemListKeepsForeignIEs(t *testing.T) {
	for _, c := range []struct {
		name            string
		fields          []FieldValue[uint16]
		values          []uint8
		foreign         int
		notComprehended bool
	}{
		{"front", []FieldValue[uint16]{foreignField(CriticalityReject), typedField(1), typedField(2)}, []uint8{1, 2}, 1, false},
		{"middle", []FieldValue[uint16]{typedField(1), foreignField(CriticalityIgnore), typedField(2)}, []uint8{1, 2}, 1, false},
		{"end", []FieldValue[uint16]{typedField(1), foreignField(CriticalityNotify), foreignField(CriticalityIgnore)}, []uint8{1}, 2, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := encodeTestList(t, c.fields)

			items, err := DecodeItemList[uint16, testItem](testProfile, per.NewReader(b), per.Aligned, testListUB, testItemID, CriticalityIgnore)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			var got []uint8
			for _, it := range items {
				got = append(got, it.V)
			}

			if !slices.Equal(got, c.values) {
				t.Errorf("typed values = %v, want %v", got, c.values)
			}

			var d Diagnosis[uint16]

			if nc := DiagnoseItemList(&d, testItemID, CriticalityIgnore, &items); nc != c.notComprehended {
				t.Errorf("not comprehended = %v, want %v", nc, c.notComprehended)
			}

			var reported []Diagnostic[uint16]

			for _, e := range d.Diagnostics().IEs {
				if e.ID == testForeignID && e.Kind == KindNotComprehended {
					reported = append(reported, e)
				}
			}

			if len(reported) != c.foreign {
				t.Fatalf("reported %v, want %d foreign IEs", d.Diagnostics().IEs, c.foreign)
			}

			i := 0

			for _, f := range c.fields {
				if f.ID == testForeignID {
					if reported[i].Criticality != f.Criticality {
						t.Errorf("foreign IE %d reported with %s, sent with %s", i, reported[i].Criticality, f.Criticality)
					}

					i++
				}
			}

			w := per.NewWriter()
			if err := EncodeItemList(testProfile, w, per.Aligned, testListUB, testItemID, CriticalityIgnore, items); err != nil {
				t.Fatalf("encode: %v", err)
			}

			w.AlignToByte()

			if !bytes.Equal(w.Bytes(), b) {
				t.Fatalf("re-encode = % x, want % x", w.Bytes(), b)
			}
		})
	}
}

func TestItemListForeignIEsAreCheckedOnTheirOwnCriticality(t *testing.T) {
	for _, c := range []struct {
		crit   Criticality
		reject bool
	}{
		{CriticalityIgnore, false},
		{CriticalityNotify, false},
		{CriticalityReject, true},
	} {
		t.Run(c.crit.String(), func(t *testing.T) {
			b := encodeTestList(t, []FieldValue[uint16]{typedField(1), foreignField(c.crit)})

			items, err := DecodeItemList[uint16, testItem](testProfile, per.NewReader(b), per.Aligned, testListUB, testItemID, CriticalityIgnore)
			if err != nil {
				t.Fatal(err)
			}

			var d Diagnosis[uint16]

			list := itemListValue(items)

			d.IE(1, CriticalityIgnore, &list)

			if err := d.Diagnostics().Check(); (err != nil) != c.reject {
				t.Fatalf("Check() = %v, want reject %v", err, c.reject)
			}

			for _, e := range d.Diagnostics().IEs {
				if e.ID == testForeignID && !slices.Equal(e.Path, []uint16{1}) {
					t.Errorf("foreign IE path = %v, want [1]", e.Path)
				}
			}
		})
	}
}

type itemListValue []testItem

func (l *itemListValue) Diagnose(d *Diagnosis[uint16]) bool {
	return DiagnoseItemList(d, testItemID, CriticalityIgnore, (*[]testItem)(l))
}

func TestItemListNotComprehendedItemIsJudgedOnItsOwnCriticality(t *testing.T) {
	bad := FieldValue[uint16]{ID: testItemID, Criticality: CriticalityReject, Value: &testItem{V: notComprehendedOctet}}

	for _, c := range []struct {
		name       string
		fields     []FieldValue[uint16]
		values     []uint8
		pruned     []uint8
		reject     bool
		listNotCmp bool
	}{
		{"ignore item", []FieldValue[uint16]{typedField(1), typedField(notComprehendedOctet), typedField(2)}, []uint8{1, notComprehendedOctet, 2}, []uint8{1, 2}, false, false},
		{"reject item", []FieldValue[uint16]{typedField(1), bad}, []uint8{1, notComprehendedOctet}, []uint8{1}, true, false},
		{"every item", []FieldValue[uint16]{typedField(notComprehendedOctet)}, []uint8{notComprehendedOctet}, []uint8{notComprehendedOctet}, false, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := encodeTestList(t, c.fields)

			items, err := DecodeItemList[uint16, testItem](testProfile, per.NewReader(b), per.Aligned, testListUB, testItemID, CriticalityIgnore)
			if err != nil {
				t.Fatal(err)
			}

			if got := values(items); !slices.Equal(got, c.values) {
				t.Fatalf("decoded values = %v, want %v: decode must keep every item", got, c.values)
			}

			list := itemListValue(items)

			var d Diagnosis[uint16]

			if nc := d.IE(1, CriticalityIgnore, &list); nc != c.listNotCmp {
				t.Fatalf("list not comprehended = %v, want %v", nc, c.listNotCmp)
			}

			if err := d.Diagnostics().Check(); (err != nil) != c.reject {
				t.Fatalf("Check() = %v, want reject %v", err, c.reject)
			}

			pruned := itemListValue(slices.Clone(items))

			Prune[uint16](&pruned)

			if got := values(pruned); !slices.Equal(got, c.pruned) {
				t.Fatalf("pruned values = %v, want %v", got, c.pruned)
			}

			w := per.NewWriter()
			if err := EncodeItemList(testProfile, w, per.Aligned, testListUB, testItemID, CriticalityIgnore, []testItem(pruned)); err != nil {
				t.Fatal(err)
			}

			w.AlignToByte()

			if !bytes.Equal(w.Bytes(), b) {
				t.Fatalf("pruned list re-encoded %x, want %x", w.Bytes(), b)
			}
		})
	}
}

func values(items []testItem) []uint8 {
	out := make([]uint8, len(items))
	for i, it := range items {
		out[i] = it.V
	}

	return out
}

func TestItemListOfOnlyForeignIEsHasNoTypedValue(t *testing.T) {
	fields := []FieldValue[uint16]{foreignField(CriticalityIgnore), foreignField(CriticalityReject)}

	items, err := DecodeItemList[uint16, testItem](testProfile, per.NewReader(encodeTestList(t, fields)), per.Aligned, testListUB, testItemID, CriticalityIgnore)

	inner, ok := asUncomprehended[uint16](err)
	if !ok || items != nil || len(inner) != 2 || inner[1].Criticality != CriticalityReject {
		t.Fatalf("DecodeItemList = %v, %v, want no typed items and the two raw IEs", items, err)
	}
}
