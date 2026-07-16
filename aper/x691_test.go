// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/aper"
)

// TestX691AnnexAEmployeeNumber pins the one field of the Rec. ITU-T X.691
// Annex A "PersonnelRecord" that this codec can encode standalone: the
// EmployeeNumber, an unconstrained INTEGER with value 51. Both the aligned
// (A.1.3) and unaligned (A.1.4) representations encode it identically as the
// length determinant 01 followed by the content octet 33.
//
// The rest of the Annex A record needs PER permitted-alphabet character
// strings and SET/DEFAULT handling, which no 3GPP protocol this codec serves
// uses; it is deliberately not implemented.
func TestX691AnnexAEmployeeNumber(t *testing.T) {
	want := []byte{0x01, 0x33}

	for _, variant := range []struct {
		name string
		w    func() *aper.Writer
	}{
		{"aligned", func() *aper.Writer { return &aper.Writer{} }},
		{"unaligned", aper.NewUnalignedWriter},
	} {
		t.Run(variant.name, func(t *testing.T) {
			w := variant.w()
			if err := w.WriteUnconstrainedInt(51); err != nil {
				t.Fatalf("WriteUnconstrainedInt: %v", err)
			}

			if !bytes.Equal(w.Bytes(), want) {
				t.Errorf("EmployeeNumber 51: got % x, want % x", w.Bytes(), want)
			}
		})
	}
}

// TestX691ConstrainedIntBitFieldSizes pins the UNALIGNED constrained whole
// number encoding against Rec. ITU-T X.691 §11.5.6 (value encoded in the
// minimum number of bits for the range) and the §11.5.7.1 size table. The value
// occupies the most-significant bits; the trailing bits of the final octet are
// zero-padded. This is the rule the whole LPP codec rests on.
func TestX691ConstrainedIntBitFieldSizes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		v, lb, ub int64
		want      []byte // nil = empty bit-field (X.691 §11.5.4, range 1)
	}{
		{"range1 no bits", 7, 7, 7, nil},
		{"range2 1bit", 1, 0, 1, []byte{0x80}},        // 1
		{"range3 2bits", 2, 0, 2, []byte{0x80}},       // 10
		{"range4 2bits", 3, 0, 3, []byte{0xC0}},       // 11
		{"range8 3bits", 5, 0, 7, []byte{0xA0}},       // 101
		{"range128 7bits", 100, 0, 127, []byte{0xC8}}, // 1100100 + pad
		{"range255 8bits", 254, 0, 254, []byte{0xFE}}, // 11111110
		{"range256 8bits", 200, 0, 255, []byte{0xC8}}, // 11001000
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := aper.NewUnalignedWriter()
			if err := w.WriteConstrainedInt(tc.v, tc.lb, tc.ub); err != nil {
				t.Fatalf("encode: %v", err)
			}

			if !bytes.Equal(w.Bytes(), tc.want) {
				t.Fatalf("encode: got % x, want % x", w.Bytes(), tc.want)
			}

			r := aper.NewUnalignedReader(w.Bytes())

			got, err := r.ReadConstrainedInt(tc.lb, tc.ub)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			if got != tc.v {
				t.Errorf("round-trip: got %d, want %d", got, tc.v)
			}
		})
	}
}

// TestX691ConstrainedIntAlignedOctetForms pins the ALIGNED constrained whole
// number encoding used by S1AP/NGAP/NRPPa against Rec. ITU-T X.691 §11.5.7.2
// (range exactly 256, one octet) and §11.5.7.3 (range 257..64K, two octets).
func TestX691ConstrainedIntAlignedOctetForms(t *testing.T) {
	for _, tc := range []struct {
		name      string
		v, lb, ub int64
		want      []byte
	}{
		{"range256 one octet", 200, 0, 255, []byte{0xC8}},
		{"range1000 two octets", 260, 0, 999, []byte{0x01, 0x04}},
		{"range300 two octets low", 1, 0, 299, []byte{0x00, 0x01}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var w aper.Writer
			if err := w.WriteConstrainedInt(tc.v, tc.lb, tc.ub); err != nil {
				t.Fatalf("encode: %v", err)
			}

			if !bytes.Equal(w.Bytes(), tc.want) {
				t.Errorf("encode: got % x, want % x", w.Bytes(), tc.want)
			}
		})
	}
}
