// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import (
	"bytes"
	"testing"
)

// Known vectors pinning the aligned-variant encodings NGAP depends on and
// S1AP does not (TS 38.413 / X.691).

// AMF-UE-NGAP-ID is INTEGER (0..2^40-1), wider than any S1AP identifier and
// wider than the four-octet case §11.5.7.4 covers with a single length octet.
func TestAlignedWideConstrainedIntKnownVectors(t *testing.T) {
	const amfUENGAPIDMax = 1099511627775 // 2^40-1

	// Range > 64K, so the value carries a length determinant giving its octet
	// count and is then octet-aligned (§11.5.7.4). The largest value needs
	// five octets, so the determinant is a constrained whole number in 1..5 —
	// three bits, where a 32-bit identifier's 1..4 needs only two. That width
	// is the reason these vectors exist: it is the one thing a 40-bit id
	// changes about the encoding.
	cases := []struct {
		name string
		v    int64
		want []byte
	}{
		{"zero", 0, []byte{0x00, 0x00}},
		{"one", 1, []byte{0x00, 0x01}},
		{"two octets", 0x0100, []byte{0x20, 0x01, 0x00}},
		{"three octets", 0x010000, []byte{0x40, 0x01, 0x00, 0x00}},
		{"four octets", 0x01000000, []byte{0x60, 0x01, 0x00, 0x00, 0x00}},
		// The fifth octet is what a 32-bit identifier never reaches.
		{"five octets", 0x0100000000, []byte{0x80, 0x01, 0x00, 0x00, 0x00, 0x00}},
		{"maximum", amfUENGAPIDMax, []byte{0x80, 0xff, 0xff, 0xff, 0xff, 0xff}},
	}

	b := Bounds{LB: 0, HasLB: true, UB: amfUENGAPIDMax, HasUB: true}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := NewWriter()
			if err := EncodeInteger(w, Aligned, b, c.v); err != nil {
				t.Fatalf("encode: %v", err)
			}

			w.AlignToByte()

			if got := w.Bytes(); !bytes.Equal(got, c.want) {
				t.Fatalf("bytes = % x, want % x", got, c.want)
			}

			got, err := DecodeInteger(NewReader(w.Bytes()), Aligned, b)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			if got != c.v {
				t.Fatalf("decoded %d, want %d", got, c.v)
			}
		})
	}
}

// BitRate ::= INTEGER (0..4000000000000, ...) is extensible, so a value inside
// the root range costs a leading 0 bit before the constrained encoding.
func TestExtensibleBitRateKnownVectors(t *testing.T) {
	b := Bounds{LB: 0, HasLB: true, UB: 4000000000000, HasUB: true, Extensible: true}

	for _, v := range []int64{0, 1, 1 << 20, 4000000000000} {
		w := NewWriter()
		if err := EncodeInteger(w, Aligned, b, v); err != nil {
			t.Fatalf("%d: encode: %v", v, err)
		}

		w.AlignToByte()

		if len(w.Bytes()) == 0 {
			t.Fatalf("%d: encoded to nothing", v)
		}

		// The extension marker is the first bit and must be clear in the root.
		if w.Bytes()[0]&0x80 != 0 {
			t.Errorf("%d: extension bit set for a root value", v)
		}

		got, err := DecodeInteger(NewReader(w.Bytes()), Aligned, b)
		if err != nil {
			t.Fatalf("%d: decode: %v", v, err)
		}

		if got != v {
			t.Errorf("decoded %d, want %d", got, v)
		}
	}
}

// The GUAMI fields are BIT STRINGs of 8, 10 and 6 bits. Only the 8-bit one is
// octet-aligned, so the other two pin the sub-octet bit-field path (§16.9).
func TestGUAMIBitStringKnownVectors(t *testing.T) {
	cases := []struct {
		name  string
		nbits int
		data  []byte
		want  []byte
	}{
		{"AMFRegionID 8 bits", 8, []byte{0x02}, []byte{0x02}},
		{"AMFSetID 10 bits", 10, []byte{0x00, 0x40}, []byte{0x00, 0x40}},
		{"AMFPointer 6 bits", 6, []byte{0x0c}, []byte{0x0c}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := NewWriter()

			n := int64(c.nbits)
			if err := EncodeBitString(w, Aligned, n, n, true, true, false, c.data, c.nbits); err != nil {
				t.Fatalf("encode: %v", err)
			}

			w.AlignToByte()

			if got := w.Bytes(); !bytes.Equal(got, c.want) {
				t.Fatalf("bytes = % x, want % x", got, c.want)
			}

			got, bits, err := DecodeBitString(NewReader(w.Bytes()), Aligned, n, n, true, true, false)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			if bits != c.nbits || !bytes.Equal(got, c.data) {
				t.Fatalf("decoded % x/%d bits, want % x/%d", got, bits, c.data, c.nbits)
			}
		})
	}
}

// The gNB-ID is BIT STRING (SIZE(22..32)): a constrained but variable size, so
// the encoding carries a length determinant the fixed-size cases do not.
func TestGNBIDVariableBitStringRoundTrip(t *testing.T) {
	for nbits := 22; nbits <= 32; nbits++ {
		data := make([]byte, (nbits+7)/8)
		for i := range data {
			data[i] = 0xff
		}
		// Clear the bits past nbits so the value is canonical.
		if rem := nbits % 8; rem != 0 {
			data[len(data)-1] &= byte(0xff << (8 - rem))
		}

		w := NewWriter()
		if err := EncodeBitString(w, Aligned, 22, 32, true, true, false, data, nbits); err != nil {
			t.Fatalf("%d bits: encode: %v", nbits, err)
		}

		w.AlignToByte()

		got, bits, err := DecodeBitString(NewReader(w.Bytes()), Aligned, 22, 32, true, true, false)
		if err != nil {
			t.Fatalf("%d bits: decode: %v", nbits, err)
		}

		if bits != nbits || !bytes.Equal(got, data) {
			t.Fatalf("%d bits: decoded % x/%d, want % x", nbits, got, bits, data)
		}
	}
}
