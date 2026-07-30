// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import (
	"bytes"
	"errors"
	"testing"
)

// Known vectors ported from s1ap/aper, pinning the aligned-variant encodings
// S1AP depends on (TS 36.413 / X.691).

func TestAlignedConstrainedIntKnownVectors(t *testing.T) {
	cases := []struct {
		name      string
		v, lb, ub int64
		want      []byte
	}{
		{"single value", 7, 7, 7, nil},
		{"one bit set", 1, 0, 1, []byte{0x80}},
		{"one bit clear", 0, 0, 1, []byte{0x00}},
		{"one octet", 5, 0, 255, []byte{0x05}},
		{"two octets", 0x0102, 0, 65535, []byte{0x01, 0x02}},
		// range > 64K: octet-count + aligned value (§11.5.7.4). MME-UE-S1AP-ID
		// is INTEGER(0..4294967295).
		{"mme-ue-id 1", 1, 0, 4294967295, []byte{0x00, 0x01}},
		{"mme-ue-id 256", 256, 0, 4294967295, []byte{0x40, 0x01, 0x00}},
		// ENB-UE-S1AP-ID is INTEGER(0..16777215): range 2^24, 3 octets max.
		{"enb-ue-id 1", 1, 0, 16777215, []byte{0x00, 0x01}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := Bounds{LB: c.lb, HasLB: true, UB: c.ub, HasUB: true}

			w := NewWriter()
			if err := EncodeInteger(w, Aligned, b, c.v); err != nil {
				t.Fatalf("encode: %v", err)
			}

			w.AlignToByte()

			if !bytes.Equal(w.Bytes(), c.want) && (len(w.Bytes()) != 0 || len(c.want) != 0) {
				t.Fatalf("bytes = % x, want % x", w.Bytes(), c.want)
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

func TestAlignedConstrainedIntUnalignedStartRoundTrip(t *testing.T) {
	ranges := []struct{ lb, ub int64 }{
		{0, 1},
		{0, 7},
		{0, 254},
		{0, 255},
		{0, 256},
		{3, 20},
		{0, 65535},
		{0, 65536},
		{0, 16777215},
		{0, 4294967295},
		{-128, 127},
		{100, 1 << 33},
	}
	for _, rg := range ranges {
		for _, v := range []int64{rg.lb, rg.lb + 1, (rg.lb + rg.ub) / 2, rg.ub} {
			b := Bounds{LB: rg.lb, HasLB: true, UB: rg.ub, HasUB: true}

			// A stray leading bit exercises the alignment paths.
			w := NewWriter()
			w.WriteBit(true)

			if err := EncodeInteger(w, Aligned, b, v); err != nil {
				t.Fatalf("encode v=%d [%d,%d]: %v", v, rg.lb, rg.ub, err)
			}

			w.AlignToByte()
			r := NewReader(w.Bytes())

			if _, err := r.ReadBit(); err != nil {
				t.Fatal(err)
			}

			got, err := DecodeInteger(r, Aligned, b)
			if err != nil {
				t.Fatalf("decode v=%d [%d,%d]: %v", v, rg.lb, rg.ub, err)
			}

			if got != v {
				t.Fatalf("v=%d [%d,%d]: decoded %d", v, rg.lb, rg.ub, got)
			}
		}
	}
}

func TestConstrainedDecodeRejectsUnassignedPatterns(t *testing.T) {
	// Range 3 (lb=0, ub=2) uses a 2-bit field; the pattern 0b11 has no
	// assigned value (§11.5) and must be rejected, not returned as lb+3.
	if _, err := DecodeConstrainedWholeNumber(NewReader([]byte{0xc0}), Aligned, 0, 2); !errors.Is(err, ErrOverflow) {
		t.Fatalf("bit-field: err = %v, want ErrOverflow", err)
	}

	// Two-octet case: range 300, value 300 out of range.
	if _, err := DecodeConstrainedWholeNumber(NewReader([]byte{0x01, 0x2c}), Aligned, 0, 299); !errors.Is(err, ErrOverflow) {
		t.Fatalf("two-octet: err = %v, want ErrOverflow", err)
	}

	// Indefinite case: 4-octet value 2^32-1 against INTEGER(0..2^31).
	in := []byte{0xc0, 0xff, 0xff, 0xff, 0xff}
	if _, err := DecodeInteger(NewReader(in), Aligned, Bounds{LB: 0, HasLB: true, UB: 1 << 31, HasUB: true}); !errors.Is(err, ErrOverflow) {
		t.Fatalf("indefinite: err = %v, want ErrOverflow", err)
	}
}

func TestEnumeratedKnownVectors(t *testing.T) {
	// Criticality ::= ENUMERATED { reject, ignore, notify } (3 values, not
	// extensible) encodes as a 2-bit index.
	cases := []struct {
		index int64
		want  []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x40}},
		{2, []byte{0x80}},
	}
	for _, c := range cases {
		w := NewWriter()
		if err := EncodeEnumerated(w, Aligned, 3, false, c.index); err != nil {
			t.Fatalf("index %d: %v", c.index, err)
		}

		w.AlignToByte()

		if !bytes.Equal(w.Bytes(), c.want) {
			t.Fatalf("index %d: bytes = % x, want % x", c.index, w.Bytes(), c.want)
		}

		got, err := DecodeEnumerated(NewReader(w.Bytes()), Aligned, 3, false)
		if err != nil || got != c.index {
			t.Fatalf("index %d: decoded %d err=%v", c.index, got, err)
		}
	}
}

func TestEnumeratedExtensionRoundTrip(t *testing.T) {
	// Extensible enum with 3 root values; the first extension addition has
	// index 3 (nRoot+0) and encodes as ext bit + normally-small 0.
	w := NewWriter()
	if err := EncodeEnumerated(w, Aligned, 3, true, 3); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	got, err := DecodeEnumerated(NewReader(w.Bytes()), Aligned, 3, true)
	if err != nil || got != 3 {
		t.Fatalf("decoded %d err=%v", got, err)
	}

	// A root value of an extensible enum must reject the unassigned root
	// pattern 0b11.
	if _, err := DecodeEnumerated(NewReader([]byte{0x60}), Aligned, 3, true); !errors.Is(err, ErrOverflow) {
		t.Fatalf("unassigned root pattern: err = %v, want ErrOverflow", err)
	}
}

func TestEnumeratedRejectsExtensionWhenNotExtensible(t *testing.T) {
	if err := EncodeEnumerated(NewWriter(), Aligned, 3, false, 3); !errors.Is(err, ErrOverflow) {
		t.Fatalf("err = %v, want ErrOverflow", err)
	}
}

func TestOpenTypeEmptyEncodesZeroOctet(t *testing.T) {
	// §11.2.1: an open type field contains at least one octet; an empty inner
	// encoding (e.g. NULL) becomes a single zero octet.
	w := NewWriter()
	if err := EncodeOpenTypeBytes(w, Aligned, nil); err != nil {
		t.Fatal(err)
	}

	if want := []byte{0x01, 0x00}; !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("bytes = % x, want % x", w.Bytes(), want)
	}
}

func TestOpenTypeBytesRoundTrip(t *testing.T) {
	for _, content := range [][]byte{{0xab}, {0xab, 0xcd}, bytes.Repeat([]byte{0x5a}, 200)} {
		w := NewWriter()
		if err := EncodeOpenTypeBytes(w, Aligned, content); err != nil {
			t.Fatal(err)
		}

		got, err := DecodeOpenTypeBytes(NewReader(w.Bytes()), Aligned)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got, content) {
			t.Fatalf("round-trip = % x, want % x", got, content)
		}
	}
}

// TestS1APEnvelopeLayout composes the primitives into the S1AP-PDU
// InitiatingMessage framing (TS 36.413) and pins the exact byte layout.
func TestS1APEnvelopeLayout(t *testing.T) {
	w := NewWriter()

	// S1AP-PDU CHOICE -> initiatingMessage (idx 0 of 3, extensible):
	// ext bit 0 + 2-bit index 00.
	w.WriteBit(false)

	if err := EncodeConstrainedWholeNumber(w, Aligned, 0, 2, 0); err != nil {
		t.Fatal(err)
	}
	// InitiatingMessage SEQUENCE: not extensible, no optional fields — no
	// preamble bits. procedureCode INTEGER(0..255) = 17 (s1Setup).
	if err := EncodeInteger(w, Aligned, Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}, 17); err != nil {
		t.Fatal(err)
	}
	// criticality ENUMERATED(3) = reject (0).
	if err := EncodeEnumerated(w, Aligned, 3, false, 0); err != nil {
		t.Fatal(err)
	}
	// value open type carrying one octet.
	if err := EncodeOpenTypeBytes(w, Aligned, []byte{0xab}); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	want := []byte{0x00, 0x11, 0x00, 0x01, 0xab}
	if !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("envelope = % x, want % x", w.Bytes(), want)
	}
}

func FuzzAlignedDecodeNoPanic(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte{0x80, 0x01})
	f.Add([]byte{0x40, 0x01, 0x00})
	f.Add([]byte{0xbf, 0xff, 0xde, 0xad, 0xbe, 0xef})

	f.Fuzz(func(t *testing.T, data []byte) {
		r := NewReader(data)
		_, _ = DecodeInteger(r, Aligned, Bounds{LB: 0, HasLB: true, UB: 4294967295, HasUB: true})
		_, _ = DecodeInteger(r, Aligned, Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true})
		_, _ = DecodeNormallySmall(r, Aligned)
		_, _ = DecodeEnumerated(r, Aligned, 3, true)
		_, _ = DecodeOpenTypeBytes(r, Aligned)
		_, _ = DecodeOctetString(r, Aligned, 0, 0, false, false, false)
		_, _ = DecodeOctetString(r, Aligned, 3, 3, true, true, false)
		_, _, _ = DecodeBitString(r, Aligned, 1, 160, true, true, true)
	})
}
