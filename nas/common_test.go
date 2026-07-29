// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

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

	w.LV([]byte{0x01, 0x02, 0x03})
	w.LVE([]byte{0xde, 0xad, 0xbe, 0xef})

	encoded, err := w.Bytes()
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	r := NewReader(encoded)

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

	w.LV(make([]byte, 256))

	if _, err := w.Bytes(); !errors.Is(err, ErrOverflow) {
		t.Fatalf("LV(256) err = %v, want ErrOverflow", err)
	}
}

// TestWriterPoisoned confirms a framing failure suppresses every later write, so
// a poisoned Writer never yields a partially-encoded message.
func TestWriterPoisoned(t *testing.T) {
	var w Writer

	w.U8(0x01)
	w.LV(make([]byte, 256))
	w.U8(0x02)

	raw, err := w.Bytes()
	if !errors.Is(err, ErrOverflow) {
		t.Fatalf("err = %v, want ErrOverflow", err)
	}

	if !bytes.Equal(raw, []byte{0x01}) {
		t.Fatalf("buffer = %#x, want only the octets written before the failure", raw)
	}
}

// TestWriterAppendsToCaller confirms NewWriter appends to the caller's buffer,
// which is what makes AppendBinary composable.
func TestWriterAppendsToCaller(t *testing.T) {
	w := NewWriter([]byte{0xAA})

	w.U8(0xBB)

	raw, err := w.Bytes()
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	if !bytes.Equal(raw, []byte{0xAA, 0xBB}) {
		t.Fatalf("buffer = %#x, want aabb", raw)
	}
}

// TestLVFuncMeasuresContent confirms the length-prefixed sub-writer emits the
// length of what the callback produced.
func TestLVFuncMeasuresContent(t *testing.T) {
	var w Writer

	w.LVFunc(func(c *Writer) {
		c.U8(0xDE)
		c.U8(0xAD)
	})

	raw, err := w.Bytes()
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	if !bytes.Equal(raw, []byte{0x02, 0xDE, 0xAD}) {
		t.Fatalf("buffer = %#x, want 02dead", raw)
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
	enc, err := PLMN{MCC: "001", MNC: "01"}.Octets()
	if err != nil {
		t.Fatal(err)
	}

	if enc != [3]byte{0x00, 0xf1, 0x10} {
		t.Fatalf("PLMN{001,01}.Octets() = % x, want 00 f1 10", enc)
	}

	for _, want := range []PLMN{{"001", "01"}, {"302", "720"}, {"310", "260"}} {
		b, err := want.Octets()
		if err != nil {
			t.Fatal(err)
		}

		got, err := ParsePLMN(b)
		if err != nil {
			t.Fatalf("decode % x: %v", b, err)
		}

		if got != want {
			t.Fatalf("round-trip %s -> % x -> %s", want, b, got)
		}

		if !want.Valid() {
			t.Errorf("%s reported invalid", want)
		}
	}

	// An identity whose digit counts are wrong has no encoding.
	for _, bad := range []PLMN{{"01", "01"}, {"001", "1"}, {"001", "0001"}, {"00a", "01"}} {
		if bad.Valid() {
			t.Errorf("%+v reported valid", bad)
		}

		if _, err := bad.Octets(); err == nil {
			t.Errorf("%+v encoded", bad)
		}
	}
}

// TestParsePLMNRejectsNonDecimal confirms an identity carrying a nibble outside
// 0-9 is rejected rather than decoded into digits that cannot be re-encoded
// (TS 23.003 §2.2).
func TestParsePLMNRejectsNonDecimal(t *testing.T) {
	for _, b := range [][3]byte{
		{0xAB, 0xF1, 0x23}, // MCC nibbles 0xB, 0xA
		{0x12, 0xF3, 0xEE}, // MNC nibbles 0xE
		{0x12, 0xA3, 0x45}, // MNC digit 3 = 0xA, neither decimal nor the filler
	} {
		if p, err := ParsePLMN(b); err == nil {
			t.Errorf("ParsePLMN(% x) = %s, want an error", b, p)
		}
	}
}
