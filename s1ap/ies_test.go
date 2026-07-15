// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/aper"
)

func TestEncodeIEContainerKnownVector(t *testing.T) {
	var w aper.Writer

	err := encodeIEContainer(&w, []ieField{{
		id:   59,
		crit: CriticalityIgnore,
		enc: func(vw *aper.Writer) error {
			vw.WriteOctets([]byte{0xab, 0xcd})
			return nil
		},
	}})
	if err != nil {
		t.Fatal(err)
	}

	// count=1 (00 01), id=59 (00 3b), criticality ignore=01 then open-type
	// length (40 02) + content (ab cd).
	want := []byte{0x00, 0x01, 0x00, 0x3b, 0x40, 0x02, 0xab, 0xcd}
	if !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("encoded % x, want % x", w.Bytes(), want)
	}
}

func TestEmptyIEContainer(t *testing.T) {
	var w aper.Writer

	if err := encodeIEContainer(&w, nil); err != nil {
		t.Fatal(err)
	}

	if want := []byte{0x00, 0x00}; !bytes.Equal(w.Bytes(), want) {
		t.Fatalf("empty container = % x, want % x", w.Bytes(), want)
	}

	got, err := decodeIEContainer(aper.NewReader(w.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 0 {
		t.Fatalf("decoded %d IEs, want 0", len(got))
	}
}

func TestIEContainerRoundTrip(t *testing.T) {
	in := []ieField{
		{id: 59, crit: CriticalityReject, enc: octetsEnc([]byte{0x01, 0x02, 0x03})},
		{id: 1, crit: CriticalityIgnore, enc: octetsEnc([]byte{0xff})},
		{id: 65535, crit: CriticalityNotify, enc: octetsEnc(nil)},
	}

	var w aper.Writer
	if err := encodeIEContainer(&w, in); err != nil {
		t.Fatal(err)
	}

	got, err := decodeIEContainer(aper.NewReader(w.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != len(in) {
		t.Fatalf("decoded %d IEs, want %d", len(got), len(in))
	}

	wantValues := [][]byte{{0x01, 0x02, 0x03}, {0xff}, {0x00}} // empty body -> open type min 1 octet

	for i, f := range in {
		if got[i].id != f.id || got[i].crit != f.crit {
			t.Fatalf("IE %d: id/crit mismatch got {%d,%v}", i, got[i].id, got[i].crit)
		}

		if !bytes.Equal(got[i].value, wantValues[i]) {
			t.Fatalf("IE %d: value % x, want % x", i, got[i].value, wantValues[i])
		}
	}
}

func TestRawIEReencodePreserves(t *testing.T) {
	// A container decoded and re-encoded from preserved raw IEs is byte-identical.
	original := []ieField{
		{id: 10, crit: CriticalityReject, enc: octetsEnc([]byte{0xde, 0xad})},
		{id: 20, crit: CriticalityIgnore, enc: octetsEnc([]byte{0xbe, 0xef, 0x00})},
	}

	var w1 aper.Writer
	if err := encodeIEContainer(&w1, original); err != nil {
		t.Fatal(err)
	}

	raw, err := decodeIEContainer(aper.NewReader(w1.Bytes()))
	if err != nil {
		t.Fatal(err)
	}

	fields := make([]ieField, len(raw))
	for i, e := range raw {
		fields[i] = e.field()
	}

	var w2 aper.Writer
	if err := encodeIEContainer(&w2, fields); err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(w1.Bytes(), w2.Bytes()) {
		t.Fatalf("re-encode differs:\n  first  % x\n  second % x", w1.Bytes(), w2.Bytes())
	}
}

func TestDecodeIEContainerTruncated(t *testing.T) {
	// Claims one IE but provides no field body.
	if _, err := decodeIEContainer(aper.NewReader([]byte{0x00, 0x01})); err == nil {
		t.Fatal("expected truncation error")
	}

	// Claims 65535 IEs in a 2-byte packet: must fail fast, not over-allocate.
	if _, err := decodeIEContainer(aper.NewReader([]byte{0xff, 0xff})); err == nil {
		t.Fatal("expected truncation error for oversized count")
	}
}

func octetsEnc(b []byte) func(*aper.Writer) error {
	return func(w *aper.Writer) error {
		w.WriteOctets(b)
		return nil
	}
}
