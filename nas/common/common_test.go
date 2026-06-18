// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import (
	"bytes"
	"errors"
	"testing"
)

func TestReaderWriterRoundTrip(t *testing.T) {
	var w Writer

	w.U8(0x07)
	w.U16(0x3039)
	w.Raw([]byte{0xaa, 0xbb})

	if err := w.LV([]byte{0x01, 0x02, 0x03}); err != nil {
		t.Fatal(err)
	}

	if err := w.LVE([]byte{0xde, 0xad, 0xbe, 0xef}); err != nil {
		t.Fatal(err)
	}

	r := NewReader(w.Bytes())

	u8, _ := r.U8()
	u16, _ := r.U16()
	raw, _ := r.Bytes(2)
	lv, _ := r.LV()
	lve, _ := r.LVE()

	if u8 != 0x07 || u16 != 0x3039 || !bytes.Equal(raw, []byte{0xaa, 0xbb}) ||
		!bytes.Equal(lv, []byte{0x01, 0x02, 0x03}) || !bytes.Equal(lve, []byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Fatalf("round-trip mismatch: %#x %#x %x %x %x", u8, u16, raw, lv, lve)
	}

	if r.Remaining() != 0 {
		t.Fatalf("remaining = %d, want 0", r.Remaining())
	}
}

func TestReaderTruncation(t *testing.T) {
	cases := []struct {
		name string
		buf  []byte
		read func(*Reader) error
	}{
		{"u8 empty", nil, func(r *Reader) error { _, err := r.U8(); return err }},
		{"u16 short", []byte{0x01}, func(r *Reader) error { _, err := r.U16(); return err }},
		{"bytes over", []byte{0x01}, func(r *Reader) error { _, err := r.Bytes(4); return err }},
		{"lv over", []byte{0x05, 0x01}, func(r *Reader) error { _, err := r.LV(); return err }},
		{"lve over", []byte{0x00, 0x05, 0x01}, func(r *Reader) error { _, err := r.LVE(); return err }},
		{"lv no length", nil, func(r *Reader) error { _, err := r.LV(); return err }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.read(NewReader(tc.buf))
			if !errors.Is(err, ErrTruncated) {
				t.Fatalf("err = %v, want ErrTruncated", err)
			}
		})
	}
}

func TestLVOverflow(t *testing.T) {
	var w Writer
	if err := w.LV(make([]byte, 256)); !errors.Is(err, ErrOverflow) {
		t.Fatalf("LV(256) err = %v, want ErrOverflow", err)
	}
}

func TestTBCD(t *testing.T) {
	for _, digits := range []string{"", "1234", "123456789012345", "00101"} {
		enc, err := EncodeTBCD(digits)
		if err != nil {
			t.Fatalf("encode %q: %v", digits, err)
		}

		if got := DecodeTBCD(enc); got != digits {
			t.Fatalf("round-trip %q -> % x -> %q", digits, enc, got)
		}
	}

	if _, err := EncodeTBCD("12a4"); !errors.Is(err, ErrDigit) {
		t.Fatalf("EncodeTBCD non-digit err = %v, want ErrDigit", err)
	}
}

func TestPLMN(t *testing.T) {
	enc, err := EncodePLMN("001", "01")
	if err != nil {
		t.Fatal(err)
	}

	if enc != [3]byte{0x00, 0xf1, 0x10} {
		t.Fatalf("EncodePLMN(001,01) = % x, want 00 f1 10", enc)
	}

	for _, tc := range []struct{ mcc, mnc string }{{"001", "01"}, {"302", "720"}, {"310", "260"}} {
		b, err := EncodePLMN(tc.mcc, tc.mnc)
		if err != nil {
			t.Fatal(err)
		}

		if mcc, mnc := DecodePLMN(b); mcc != tc.mcc || mnc != tc.mnc {
			t.Fatalf("round-trip %s/%s -> % x -> %s/%s", tc.mcc, tc.mnc, b, mcc, mnc)
		}
	}
}
