// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

import (
	"bytes"
	"testing"
)

func TestEnumKnownVectors(t *testing.T) {
	// Criticality ::= ENUMERATED { reject, ignore, notify } (3 values, not
	// extensible) encodes as a 2-bit index.
	cases := []struct {
		index int
		want  []byte
	}{
		{0, []byte{0x00}}, // reject  -> 00
		{1, []byte{0x40}}, // ignore  -> 01 (high bits)
		{2, []byte{0x80}}, // notify  -> 10
	}
	for _, c := range cases {
		var w Writer

		if err := w.WriteEnum(c.index, 3, false, false); err != nil {
			t.Fatalf("index %d: %v", c.index, err)
		}

		if !bytes.Equal(w.Bytes(), c.want) {
			t.Fatalf("index %d: bytes = % x, want % x", c.index, w.Bytes(), c.want)
		}

		got, isExt, err := NewReader(w.Bytes()).ReadEnum(3, false)
		if err != nil || isExt || got != c.index {
			t.Fatalf("index %d: decoded %d isExt=%v err=%v", c.index, got, isExt, err)
		}
	}
}

func TestEnumExtensionRoundTrip(t *testing.T) {
	// Extensible enum, extension value (index among extensions = 0).
	var w Writer

	if err := w.WriteEnum(0, 3, true, true); err != nil {
		t.Fatal(err)
	}

	got, isExt, err := NewReader(w.Bytes()).ReadEnum(3, true)
	if err != nil || !isExt || got != 0 {
		t.Fatalf("decoded %d isExt=%v err=%v", got, isExt, err)
	}
}

func TestChoiceIndexRoundTrip(t *testing.T) {
	// S1AP-PDU: 3 root alternatives, extensible.
	for idx := 0; idx < 3; idx++ {
		var w Writer

		if err := w.WriteChoiceIndex(idx, 3, true, false); err != nil {
			t.Fatalf("idx %d: %v", idx, err)
		}

		got, isExt, err := NewReader(w.Bytes()).ReadChoiceIndex(3, true)
		if err != nil || isExt || got != idx {
			t.Fatalf("idx %d: decoded %d isExt=%v err=%v", idx, got, isExt, err)
		}
	}

	// initiatingMessage (idx 0): ext bit 0 + index 00 = 3 bits -> 0x00.
	var w Writer

	_ = w.WriteChoiceIndex(0, 3, true, false)

	if want := []byte{0x00}; !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("idx 0 bytes = % x, want % x", w.Bytes(), want)
	}
}

func TestNSLengthRoundTrip(t *testing.T) {
	for _, n := range []int{1, 2, 63, 64, 65, 200, 16000} {
		var w Writer

		if err := w.WriteNSLength(n); err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}

		got, err := NewReader(w.Bytes()).ReadNSLength()
		if err != nil || got != n {
			t.Fatalf("n=%d: decoded %d err=%v", n, got, err)
		}
	}

	// n=64 -> (64-1)=63 in 7 bits = 0111111_0 = 0x7e.
	var w Writer

	_ = w.WriteNSLength(64)

	if want := []byte{0x7e}; !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("n=64 bytes = % x, want % x", w.Bytes(), want)
	}
}

func TestSequencePreambleRoundTrip(t *testing.T) {
	optionals := []bool{true, false, true}

	var w Writer

	w.WriteSequencePreamble(true, false, optionals)

	// ext(0) + 1 0 1 = 0101 -> 0x50.
	if want := []byte{0x50}; !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("bytes = % x, want % x", w.Bytes(), want)
	}

	extPresent, got, err := NewReader(w.Bytes()).ReadSequencePreamble(true, len(optionals))
	if err != nil {
		t.Fatal(err)
	}

	if extPresent {
		t.Fatal("extPresent should be false")
	}

	for i := range optionals {
		if got[i] != optionals[i] {
			t.Fatalf("optional %d: got %v want %v", i, got[i], optionals[i])
		}
	}
}

func TestChoiceIndexDecodeOutOfRange(t *testing.T) {
	// Non-extensible CHOICE of 3 alternatives uses 2 bits; pattern 0b11 = 3
	// has no assigned alternative and must be rejected.
	if _, _, err := NewReader([]byte{0xc0}).ReadChoiceIndex(3, false); err == nil {
		t.Fatal("expected out-of-range choice index error")
	}
}

func TestSkipExtensionAdditions(t *testing.T) {
	// Two extension additions, the first present, then a trailing value that
	// must remain readable after the additions are skipped.
	var w Writer

	if err := w.WriteNSLength(2); err != nil {
		t.Fatal(err)
	}

	w.WriteBool(true)
	w.WriteBool(false)

	if err := w.WriteOpenType([]byte{0xab, 0xcd}); err != nil {
		t.Fatal(err)
	}

	if err := w.WriteConstrainedInt(7, 0, 255); err != nil {
		t.Fatal(err)
	}

	r := NewReader(w.Bytes())
	if err := r.SkipExtensionAdditions(); err != nil {
		t.Fatalf("skip: %v", err)
	}

	v, err := r.ReadConstrainedInt(0, 255)
	if err != nil || v != 7 {
		t.Fatalf("trailing value: got %d err=%v", v, err)
	}
}

// TestEnvelopeLayout encodes an InitiatingMessage envelope by hand from the
// structured primitives and checks the exact byte layout, validating that they
// compose into the real S1AP-PDU framing (TS 36.413).
func TestEnvelopeLayout(t *testing.T) {
	var w Writer

	// S1AP-PDU CHOICE -> initiatingMessage (idx 0 of 3, extensible).
	if err := w.WriteChoiceIndex(0, 3, true, false); err != nil {
		t.Fatal(err)
	}
	// InitiatingMessage SEQUENCE: not extensible, no optional fields.
	w.WriteSequencePreamble(false, false, nil)
	// procedureCode INTEGER(0..255) = 17 (s1Setup).
	if err := w.WriteConstrainedInt(17, 0, 255); err != nil {
		t.Fatal(err)
	}
	// criticality ENUMERATED(3) = reject (0).
	if err := w.WriteEnum(0, 3, false, false); err != nil {
		t.Fatal(err)
	}
	// value open type carrying one octet.
	if err := w.WriteOpenType([]byte{0xab}); err != nil {
		t.Fatal(err)
	}

	want := []byte{0x00, 0x11, 0x00, 0x01, 0xab}
	if !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("envelope = % x, want % x", w.Bytes(), want)
	}
}
