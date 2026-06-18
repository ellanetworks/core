// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import (
	"bytes"
	"testing"
)

// recorded is one IE the walker handed back.
type recorded struct {
	iei   uint8
	value []byte
}

func walk(t *testing.T, data []byte, table []OptionalIE) ([]recorded, []byte) {
	t.Helper()

	var got []recorded

	rest, err := WalkOptionalIEs(NewReader(data), table, func(iei uint8, value []byte) error {
		got = append(got, recorded{iei, append([]byte(nil), value...)})
		return nil
	})
	if err != nil {
		t.Fatalf("WalkOptionalIEs: %v", err)
	}

	return got, rest
}

func TestWalkOptionalIEsFormats(t *testing.T) {
	table := []OptionalIE{
		{IEI: 0x19, Format: IETV3, Len: 3},
		{IEI: 0x57, Format: IETLV},
		{IEI: 0x7b, Format: IETLVE},
	}

	// type-1 TV (0xB-, value in low nibble), a TV3, a TLV, and a TLV-E in order.
	data := []byte{
		0xb2,                   // type-1 TV: IEI 0xB0, value 2
		0x19, 0x01, 0x02, 0x03, // TV3 len 3
		0x57, 0x02, 0xaa, 0xbb, // TLV len 2
		0x7b, 0x00, 0x02, 0xcc, 0xdd, // TLV-E len 2
	}

	got, rest := walk(t, data, table)
	if len(rest) != 0 {
		t.Fatalf("unexpected remainder: %x", rest)
	}

	want := []recorded{
		{0xb0, []byte{0x02}},
		{0x19, []byte{0x01, 0x02, 0x03}},
		{0x57, []byte{0xaa, 0xbb}},
		{0x7b, []byte{0xcc, 0xdd}},
	}

	if len(got) != len(want) {
		t.Fatalf("got %d IEs, want %d: %+v", len(got), len(want), got)
	}

	for i := range want {
		if got[i].iei != want[i].iei || !bytes.Equal(got[i].value, want[i].value) {
			t.Fatalf("IE %d = {%#x %x}, want {%#x %x}", i, got[i].iei, got[i].value, want[i].iei, want[i].value)
		}
	}
}

// TestWalkOptionalIEsStopsOnUnknown confirms the walk reaches a modelled IE past
// the preceding ones, and stops (returning the remainder) at the first full-octet
// IEI it cannot delimit — never guessing a length.
func TestWalkOptionalIEsStopsOnUnknown(t *testing.T) {
	table := []OptionalIE{
		{IEI: 0x52, Format: IETV3, Len: 5}, // must be skipped to reach 0x57
		{IEI: 0x57, Format: IETLV},
	}

	data := []byte{
		0x52, 1, 2, 3, 4, 5, // TV3 len 5 (skipped to reach 0x57)
		0x57, 0x02, 0x00, 0x20, // EPS bearer context status (EBI5)
		0x31, 0x01, 0xff, // unmodelled full-octet IE → walk must stop here
		0x99,
	}

	got, rest := walk(t, data, table)

	if len(got) != 2 || got[1].iei != 0x57 || !bytes.Equal(got[1].value, []byte{0x00, 0x20}) {
		t.Fatalf("expected to reach 0x57, got %+v", got)
	}

	if !bytes.Equal(rest, []byte{0x31, 0x01, 0xff, 0x99}) {
		t.Fatalf("remainder = %x, want the unmodelled tail", rest)
	}
}

// TestWalkOptionalIEsBoundedMalformed confirms a truncated TLV/TV3 returns an
// error rather than over-reading (the malformed-packet safety invariant).
func TestWalkOptionalIEsBoundedMalformed(t *testing.T) {
	table := []OptionalIE{{IEI: 0x57, Format: IETLV}}

	cases := [][]byte{
		{0x57, 0x05, 0x00}, // TLV claims 5 octets, only 1 present
		{0x57},             // IEI with no length
		{0x19},             // TV3 IEI with no value (not in table → stops, fine)
	}

	for _, data := range cases {
		_, err := WalkOptionalIEs(NewReader(data), table, func(uint8, []byte) error { return nil })
		// 0x19 is not in the table, so it stops cleanly (no error); the truncated
		// 0x57 cases must error rather than panic or over-read.
		if data[0] == 0x57 && err == nil {
			t.Fatalf("expected an error for truncated TLV %x", data)
		}
	}
}

func FuzzWalkOptionalIEs(f *testing.F) {
	f.Add([]byte{0xb2, 0x57, 0x02, 0xaa, 0xbb})
	f.Add([]byte{0x19, 0x01, 0x02, 0x03, 0x57, 0xff})
	f.Add([]byte{0x7b, 0xff, 0xff})

	table := []OptionalIE{
		{IEI: 0x19, Format: IETV3, Len: 3},
		{IEI: 0x52, Format: IETV3, Len: 5},
		{IEI: 0x57, Format: IETLV},
		{IEI: 0x7b, Format: IETLVE},
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = WalkOptionalIEs(NewReader(data), table, func(uint8, []byte) error { return nil })
	})
}
