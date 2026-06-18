// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/s1ap/aper"
)

func TestENBIDAllKinds(t *testing.T) {
	cases := []ENBID{
		{Kind: ENBIDMacro, Value: 0xabcde & 0xfffff},       // 20 bits
		{Kind: ENBIDHome, Value: 0x54f6401},                // 28 bits
		{Kind: ENBIDShortMacro, Value: 0x3abcd & 0x3ffff},  // 18 bits, extension
		{Kind: ENBIDLongMacro, Value: 0x1abcde & 0x1fffff}, // 21 bits, extension
	}
	for _, in := range cases {
		var w aper.Writer

		if err := in.encode(&w); err != nil {
			t.Fatalf("%+v: encode: %v", in, err)
		}

		got, err := decodeENBID(aper.NewReader(w.Bytes()))
		if err != nil {
			t.Fatalf("%+v: decode: %v", in, err)
		}

		if got != in {
			t.Fatalf("decoded %+v, want %+v", got, in)
		}
	}
}

func TestPagingDRXRoundTrip(t *testing.T) {
	for _, p := range []PagingDRX{PagingDRXv32, PagingDRXv64, PagingDRXv128, PagingDRXv256} {
		var w aper.Writer

		if err := p.encode(&w); err != nil {
			t.Fatal(err)
		}

		got, err := decodePagingDRX(aper.NewReader(w.Bytes()))
		if err != nil || got != p {
			t.Fatalf("p=%d: decoded %d err=%v", p, got, err)
		}
	}
}
