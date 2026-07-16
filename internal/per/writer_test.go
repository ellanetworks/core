// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import (
	"bytes"
	"testing"
)

func TestWriterWriteBit(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	// Write 0b10110010 across one byte.
	bits := []bool{true, false, true, true, false, false, true, false}
	for _, b := range bits {
		w.WriteBit(b)
	}

	if got := w.Bits(); got != 8 {
		t.Fatalf("Bits() = %d, want 8", got)
	}

	if !w.Aligned() {
		t.Fatal("expected octet-aligned after 8 bits")
	}

	got := w.Bytes()

	want := []byte{0b10110010}
	if !bytes.Equal(got, want) {
		t.Fatalf("Bytes() = %08b, want %08b", got, want)
	}
}

func TestWriterWriteBitsAcrossBoundary(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	w.WriteBits(0b101, 3)        // 0b10100000 in byte 0
	w.WriteBits(0xFFFFFFFFFF, 5) // fills 5 bits: 101_11111
	got := w.Bytes()

	want := []byte{0b10111111}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %08b, want %08b", got, want)
	}

	if w.Bits() != 8 {
		t.Fatalf("Bits() = %d, want 8", w.Bits())
	}
}

func TestWriterAlignToByte(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	w.WriteBits(0b101, 3)
	w.AlignToByte()

	if !w.Aligned() {
		t.Fatal("not aligned after AlignToByte")
	}

	w.AlignToByte()
	w.WriteBit(true)
	w.AlignToByte()
	// Expect: 0b10100000 then 0b10000000
	got := w.Bytes()

	want := []byte{0b10100000, 0b10000000}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %08b, want %08b", got, want)
	}
}

func TestWriterWriteBitString(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	// 12 bits from [0xAB, 0xC0] => 0xABC
	w.WriteBitString([]byte{0xAB, 0xC0}, 12)
	w.AlignToByte()
	got := w.Bytes()

	want := []byte{0xAB, 0xC0}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestWriterWriteOctetsRequiresAlignment(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	w.WriteBit(true)

	if err := w.WriteOctets([]byte{1}); err != ErrUnaligned {
		t.Fatalf("err = %v, want ErrUnaligned", err)
	}

	w.AlignToByte()

	if err := w.WriteOctets([]byte{1, 2, 3}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	got := w.Bytes()

	want := []byte{0x80, 1, 2, 3}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestWriterBytesPanicsWhenUnaligned(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	w.WriteBit(true)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()

	_ = w.Bytes()
}

func TestWriterBitsAccurateAcrossMany(t *testing.T) {
	t.Parallel()

	w := NewWriter()
	for range 17 {
		w.WriteBit(true)
	}

	if got := w.Bits(); got != 17 {
		t.Fatalf("Bits() = %d, want 17", got)
	}

	w.AlignToByte()

	if got := w.Bits(); got != 24 {
		t.Fatalf("Bits() = %d, want 24 after align", got)
	}
}
