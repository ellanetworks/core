// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import (
	"bytes"
	"testing"
)

func TestBuilderPrimitives(t *testing.T) {
	got := NewBuilder().
		U8(0x7e).
		U16(0x0102).
		Raw(0xaa, 0xbb).
		LV([]byte{0x01, 0x02, 0x03}).
		LVE([]byte{0xff}).
		Bytes()

	want := []byte{0x7e, 0x01, 0x02, 0xaa, 0xbb, 0x03, 0x01, 0x02, 0x03, 0x00, 0x01, 0xff}
	if !bytes.Equal(got, want) {
		t.Fatalf("got % x, want % x", got, want)
	}
}

func TestBuilderIEFormats(t *testing.T) {
	got := NewBuilder().
		TV1(0x90, 0x03).               // type-1: high nibble IEI, low nibble value
		TV(0x12, []byte{0x05}).        // type-3: IEI + fixed value
		TLV(0x2e, []byte{0xe0, 0xe0}). // type-4: IEI + 1-octet len + value
		TLVE(0x71, []byte{0x01}).      // type-6: IEI + 2-octet len + value
		Bytes()

	want := []byte{0x93, 0x12, 0x05, 0x2e, 0x02, 0xe0, 0xe0, 0x71, 0x00, 0x01, 0x01}
	if !bytes.Equal(got, want) {
		t.Fatalf("got % x, want % x", got, want)
	}
}

// TestBuilderDeliberateCorruption exercises the adversarial knobs: a declared length
// that overruns the value, and truncation mid-message.
func TestBuilderDeliberateCorruption(t *testing.T) {
	// A TLV whose declared length (0x05) exceeds the 2 value octets present.
	overrun := NewBuilder().TLVn(0x2e, 0x05, []byte{0xe0, 0xe0}).Bytes()
	if want := []byte{0x2e, 0x05, 0xe0, 0xe0}; !bytes.Equal(overrun, want) {
		t.Fatalf("overrun: got % x, want % x", overrun, want)
	}

	// Truncate keeps only the header, cutting the message mid-field.
	trunc := NewBuilder().U8(0x7e).U8(0x00).U8(0x41).U8(0x01).LVE([]byte{0xde, 0xad}).Truncate(3).Bytes()
	if want := []byte{0x7e, 0x00, 0x41}; !bytes.Equal(trunc, want) {
		t.Fatalf("truncate: got % x, want % x", trunc, want)
	}

	// Truncate past the end is a no-op (clamped).
	if got := NewBuilder().U8(0x01).Truncate(99).Bytes(); !bytes.Equal(got, []byte{0x01}) {
		t.Fatalf("over-truncate mutated the buffer: % x", got)
	}
}

// TestBuilderBytesAreCopied confirms the returned slice does not alias the builder,
// so a fuzz/table test can keep building after snapshotting.
func TestBuilderBytesAreCopied(t *testing.T) {
	b := NewBuilder().U8(0x01)
	snap := b.Bytes()
	b.U8(0x02)

	if len(snap) != 1 || snap[0] != 0x01 {
		t.Fatalf("Bytes() aliased the builder buffer: % x", snap)
	}
}
