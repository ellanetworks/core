// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestVisibleStringUnalignedBitsPerChar pins X.691 §30.5.2: in the UNALIGNED
// variant a VisibleString with no permitted-alphabet constraint uses
// B = ceil(log2(95)) = 7 bits per character, not 8. LPP (TS 37.355) carries
// EPDU-Name as VisibleString and encodes with the UNALIGNED variant, so this
// differs from a raw-octet encoding of the same value.
func TestVisibleStringUnalignedBitsPerChar(t *testing.T) {
	// "AB": 'A'=65, 'B'=66. VisibleString code values start at 32, and the
	// alphabet (95 chars) fits in 7 bits only after remapping to 0..94, so
	// 'A' → 33, 'B' → 34.
	//   0100001 0100010  = 0100_0010_1000_10xx
	//   = 0x42 0x88 (2 pad bits)
	w := NewWriter()
	if err := EncodeKnownMultiplierString(w, Unaligned, CharVisibleString, 1, 32, true, true, false, "AB"); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	// Length determinant for SIZE(1..32) is a 5-bit constrained value (n-lb=1),
	// followed by 2 characters at 7 bits each.
	got := w.Bytes()
	if len(got) != 3 {
		t.Fatalf("encoded %d octets, want 3: % x", len(got), got)
	}

	s, err := DecodeKnownMultiplierString(NewReader(got), Unaligned, CharVisibleString, 1, 32, true, true, false)
	if err != nil {
		t.Fatal(err)
	}

	if s != "AB" {
		t.Fatalf("round-trip = %q, want %q", s, "AB")
	}
	// The same value as an OCTET STRING would take 8 bits per character; pin
	// that the character encoding really is narrower.
	ow := NewWriter()
	if err := EncodeOctetString(ow, Unaligned, 1, 32, true, true, false, []byte("AB")); err != nil {
		t.Fatal(err)
	}

	ow.AlignToByte()

	if bytes.Equal(ow.Bytes(), got) {
		t.Fatal("VisibleString encoded identically to OCTET STRING; expected 7-bit characters")
	}
}

// TestKnownMultiplierStringRejectsOutOfAlphabet checks that a character the
// string type cannot represent is an error, not a panic or a silent
// substitution. A PrintableString cannot carry 'é'.
func TestKnownMultiplierStringRejectsOutOfAlphabet(t *testing.T) {
	for _, tc := range []struct {
		name string
		enc  Encoding
		typ  charStringType
		s    string
	}{
		{"printable aligned", Aligned, CharPrintableString, "é"},
		{"printable unaligned", Unaligned, CharPrintableString, "Aé"},
		{"numeric", Aligned, CharNumericString, "12x"},
		{"visible unaligned", Unaligned, CharVisibleString, "ÿ"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWriter()

			err := EncodeKnownMultiplierString(w, tc.enc, tc.typ, 1, 150, true, true, true, tc.s)
			if !errors.Is(err, ErrOverflow) {
				t.Fatalf("err = %v, want ErrOverflow", err)
			}
		})
	}
}

// TestKnownMultiplierStringMultiByteLength checks the length determinant
// counts characters, not the UTF-8 bytes backing them. BMPString admits
// non-ASCII, so a 2-rune value must encode as length 2.
func TestKnownMultiplierStringMultiByteLength(t *testing.T) {
	const s = "Aé"

	w := NewWriter()
	if err := EncodeKnownMultiplierString(w, Aligned, CharBMPString, 0, 0, false, false, false, s); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	got, err := DecodeKnownMultiplierString(NewReader(w.Bytes()), Aligned, CharBMPString, 0, 0, false, false, false)
	if err != nil {
		t.Fatal(err)
	}

	if got != s {
		t.Fatalf("round-trip = %q, want %q", got, s)
	}
}

// TestKnownMultiplierStringFragmentedContent checks that a value spanning
// more than one 16K fragment keeps its content in order. A uniform string
// cannot detect a fragment cursor that restarts at zero.
func TestKnownMultiplierStringFragmentedContent(t *testing.T) {
	s := strings.Repeat("A", fragmentUnit) + strings.Repeat("B", 100)

	w := NewWriter()
	if err := EncodeKnownMultiplierString(w, Aligned, CharIA5String, 0, 0, false, false, false, s); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	got, err := DecodeKnownMultiplierString(NewReader(w.Bytes()), Aligned, CharIA5String, 0, 0, false, false, false)
	if err != nil {
		t.Fatal(err)
	}

	if got != s {
		t.Fatalf("fragmented round-trip corrupted: len=%d want=%d, tail=%q want=%q",
			len(got), len(s), tail(got), tail(s))
	}
}

func tail(s string) string {
	if len(s) < 5 {
		return s
	}

	return s[len(s)-5:]
}
