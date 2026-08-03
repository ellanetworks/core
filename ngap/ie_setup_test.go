// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestPagingDRXRoundTrip(t *testing.T) {
	for _, p := range []PagingDRX{PagingDRXv32, PagingDRXv64, PagingDRXv128, PagingDRXv256} {
		w := per.NewWriter()

		if err := p.MarshalPER(w, per.Aligned); err != nil {
			t.Fatal(err)
		}

		got, err := unmarshalPERValue[PagingDRX](perBytes(w))
		if err != nil || got != p {
			t.Fatalf("p=%d: decoded %d err=%v", p, got, err)
		}
	}
}

// TAC is three octets in NR, where S1AP's is two, so the top octet is the part
// a 4G-shaped codec would silently drop.
func TestTACRoundTrip(t *testing.T) {
	for _, tac := range []TAC{0, 1, 0x00ffff, 0x010000, tacMax} {
		w := per.NewWriter()

		if err := tac.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("%d: encode: %v", tac, err)
		}

		if n := len(perBytes(w)); n != 3 {
			t.Errorf("%d: encoded %d octets, want 3", tac, n)
		}

		got, err := unmarshalPERValue[TAC](perBytes(w))
		if err != nil || got != tac {
			t.Fatalf("%d: decoded %d err=%v", tac, got, err)
		}
	}
}

// A value wider than three octets is refused rather than truncated.
func TestTACRejectsOversizedValue(t *testing.T) {
	w := per.NewWriter()
	if err := TAC(tacMax+1).MarshalPER(w, per.Aligned); err == nil {
		t.Fatal("encode accepted a TAC wider than three octets")
	}
}

// S-NSSAI carries SD only when present; an absent SD must stay nil rather than
// decode as three zero octets, which is a valid slice differentiator.
func TestSNSSAIOptionalSD(t *testing.T) {
	for _, in := range []SNSSAI{
		{SST: 1},
		{SST: 2, SD: &SD{0x01, 0x02, 0x03}},
		{SST: 3, SD: &SD{0x00, 0x00, 0x00}},
	} {
		w := per.NewWriter()
		if err := in.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("%+v: encode: %v", in, err)
		}

		got, err := unmarshalPERValue[SNSSAI](perBytes(w))
		if err != nil {
			t.Fatalf("%+v: decode: %v", in, err)
		}

		if got.SST != in.SST {
			t.Errorf("SST = %d, want %d", got.SST, in.SST)
		}

		if (got.SD == nil) != (in.SD == nil) {
			t.Fatalf("SD presence = %v, want %v", got.SD, in.SD)
		}

		if in.SD != nil && *got.SD != *in.SD {
			t.Errorf("SD = %v, want %v", *got.SD, *in.SD)
		}
	}
}

// The GUAMI fields are bit strings of 8, 10 and 6 bits held as integers.
func TestGUAMIRoundTrip(t *testing.T) {
	in := GUAMI{
		PLMNIdentity: goldPLMN(),
		AMFRegionID:  0xff,
		AMFSetID:     0x3ff,
		AMFPointer:   0x3f,
	}

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	got, err := unmarshalPERValue[GUAMI](perBytes(w))
	if err != nil {
		t.Fatal(err)
	}

	if got != in {
		t.Fatalf("round trip = %+v, want %+v", got, in)
	}
}

// A field wider than its bit string is an error, not a silent truncation.
func TestGUAMIRejectsOversizedFields(t *testing.T) {
	for _, in := range []GUAMI{
		{AMFSetID: 0x400},  // 11 bits into a 10-bit field
		{AMFPointer: 0x40}, // 7 bits into a 6-bit field
	} {
		w := per.NewWriter()
		if err := in.MarshalPER(w, per.Aligned); err == nil {
			t.Errorf("encode accepted %+v", in)
		}
	}
}
