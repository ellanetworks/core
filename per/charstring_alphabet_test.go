// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import (
	"errors"
	"testing"
)

func TestDecodedCharacterOutsideTheAlphabetIsRejected(t *testing.T) {
	for _, enc := range []Encoding{Aligned, Unaligned} {
		w := NewWriter()
		if err := EncodeKnownMultiplierString(w, enc, CharPrintableString, 1, 150, true, true, true, "a"); err != nil {
			t.Fatal(err)
		}

		b := w.Bytes()
		b[len(b)-1] = 0x00

		if enc == Unaligned {
			b[len(b)-1] = 0x7f
		}

		if _, err := DecodeKnownMultiplierString(NewReader(b), enc, CharPrintableString, 1, 150, true, true, true); !errors.Is(err, ErrOverflow) {
			t.Errorf("%v: decode = %v, want ErrOverflow: the encoder refuses the same value", enc, err)
		}
	}
}

func TestCompleteRejectsTrailingOctets(t *testing.T) {
	for _, c := range []struct {
		b    []byte
		read int
		ok   bool
	}{
		{[]byte{0x00}, 0, true},
		{[]byte{0x80}, 1, true},
		{[]byte{0x80, 0x00}, 1, false},
		{[]byte{0xff, 0xff}, 16, true},
		{[]byte{0xff, 0xff, 0x00}, 16, false},
	} {
		r := NewReader(c.b)
		if _, err := r.ReadBits(c.read); err != nil {
			t.Fatal(err)
		}

		if err := r.Complete(); (err == nil) != c.ok {
			t.Errorf("%x after %d bits: Complete() = %v, want ok %v", c.b, c.read, err, c.ok)
		}
	}
}

func TestEmptyOctetStringDecodesNonNil(t *testing.T) {
	w := NewWriter()
	if err := EncodeOctetString(w, Aligned, 0, 0, true, false, false, nil); err != nil {
		t.Fatal(err)
	}

	b, err := DecodeOctetString(NewReader(w.Bytes()), Aligned, 0, 0, true, false, false)
	if err != nil || b == nil || len(b) != 0 {
		t.Fatalf("DecodeOctetString = %#v, %v, want a present empty value", b, err)
	}
}
