// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import (
	"bytes"
	"testing"
)

func encodeAdditions(t *testing.T, enc Encoding, lead int, present []bool, values [][]byte) []byte {
	t.Helper()

	w := NewWriter()
	w.WriteBits(0x5, lead)

	if err := EncodeNormallySmallLength(w, enc, int64(len(present)), func(int64) error {
		for _, p := range present {
			w.WriteBit(p)
		}

		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, v := range values {
		if err := EncodeOpenTypeBytes(w, enc, v); err != nil {
			t.Fatal(err)
		}
	}

	w.WriteBits(0x3, 2)

	return w.Bytes()
}

func TestExtensionAdditionsRoundTrip(t *testing.T) {
	present := []bool{true, false, true, false, false}
	values := [][]byte{{0x01, 0x02}, bytes.Repeat([]byte{0x7f}, 300)}

	for _, enc := range []Encoding{Aligned, Unaligned} {
		for lead := range 9 {
			in := encodeAdditions(t, enc, lead, present, values)

			r := NewReader(in)
			if _, err := r.ReadBits(lead); err != nil {
				t.Fatal(err)
			}

			var e ExtensionAdditions
			if err := e.UnmarshalPER(r, enc); err != nil {
				t.Fatalf("%s lead %d: %v", enc, lead, err)
			}

			if !e.Present() || len(e.fields) != 2 {
				t.Fatalf("%s lead %d: present %v, %d fields", enc, lead, e.present, len(e.fields))
			}

			tail, err := r.ReadBits(2)
			if err != nil || tail != 0x3 {
				t.Fatalf("%s lead %d: reader misplaced after additions: %v %x", enc, lead, err, tail)
			}

			w := NewWriter()
			w.WriteBits(0x5, lead)

			if err := e.MarshalPER(w, enc); err != nil {
				t.Fatal(err)
			}

			w.WriteBits(0x3, 2)

			if out := w.Bytes(); !bytes.Equal(out, in) {
				t.Fatalf("%s lead %d: got % x, want % x", enc, lead, out, in)
			}
		}
	}
}

func TestExtensionAdditionsAbsent(t *testing.T) {
	var e ExtensionAdditions
	if e.Present() {
		t.Fatal("zero value reports present")
	}

	w := NewWriter()
	if err := e.MarshalPER(w, Aligned); err != nil || w.Bits() != 0 {
		t.Fatalf("absent additions wrote %d bits, err %v", w.Bits(), err)
	}
}

func TestExtensionAdditionsTruncated(t *testing.T) {
	in := encodeAdditions(t, Aligned, 0, []bool{true}, [][]byte{{0x01, 0x02, 0x03}})

	for i := range len(in) - 1 {
		var e ExtensionAdditions
		if err := e.UnmarshalPER(NewReader(in[:i]), Aligned); err == nil {
			t.Errorf("truncation at %d accepted", i)
		}
	}
}

func TestReaderSince(t *testing.T) {
	r := NewReader([]byte{0xab, 0xcd, 0xef})
	if _, err := r.ReadBits(3); err != nil {
		t.Fatal(err)
	}

	start := r.Offset()
	if _, err := r.ReadBits(10); err != nil {
		t.Fatal(err)
	}

	data, nbits, err := r.Since(start)
	if err != nil || nbits != 10 {
		t.Fatalf("Since: %v, %d bits", err, nbits)
	}

	if !bytes.Equal(data, []byte{0x5e, 0x40}) {
		t.Fatalf("Since = % x", data)
	}

	if _, _, err := r.Since(r.Offset() + 1); err == nil {
		t.Fatal("start past the reader accepted")
	}
}
