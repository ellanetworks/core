// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// X.691 §30.5.2: in the UNALIGNED variant a VisibleString uses
// B = ceil(log2(95)) = 7 bits per character, not 8.
func TestVisibleStringUnalignedBitsPerChar(t *testing.T) {
	// 'A'=65 and 'B'=66 remap to 33 and 34 in the 95-character alphabet:
	//   0100001 0100010, behind the 5-bit SIZE(1..32) length (n-lb = 1),
	//   = 19 bits → 3 octets.
	w := NewWriter()
	if err := EncodeKnownMultiplierString(w, Unaligned, CharVisibleString, 1, 32, true, true, false, "AB"); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

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
	// The same value as an OCTET STRING would take 8 bits per character.
	ow := NewWriter()
	if err := EncodeOctetString(ow, Unaligned, 1, 32, true, true, false, []byte("AB")); err != nil {
		t.Fatal(err)
	}

	ow.AlignToByte()

	if bytes.Equal(ow.Bytes(), got) {
		t.Fatal("VisibleString encoded identically to OCTET STRING; expected 7-bit characters")
	}
}

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

// The length determinant counts characters, not the UTF-8 bytes backing them.
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

// The mixed content matters: a uniform string could not detect a fragment
// cursor that restarts at zero.
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
