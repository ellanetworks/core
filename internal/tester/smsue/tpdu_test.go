// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsue

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestEncodeSubmitPacksGSM7(t *testing.T) {
	got, err := EncodeSubmit(0, "+15551230002", "hi")
	if err != nil {
		t.Fatal(err)
	}

	want, _ := hex.DecodeString("01000b915155210300f2000002e834")
	if !bytes.Equal(got, want) {
		t.Fatalf("SMS-SUBMIT = % x, want % x", got, want)
	}
}

func TestEncodeSubmitFallsBackToUCS2(t *testing.T) {
	got, err := EncodeSubmit(1, "15551230002", "é€")
	if err != nil {
		t.Fatal(err)
	}

	if got[len(got)-6] != dcsUCS2 || got[len(got)-5] != 4 || !bytes.Equal(got[len(got)-4:], []byte{0x00, 0xe9, 0x20, 0xac}) {
		t.Fatalf("SMS-SUBMIT = % x, want UCS2 user data", got)
	}
}

func TestDecodeDeliver(t *testing.T) {
	cases := []struct {
		name string
		tpdu string
		from string
		text string
	}{
		{name: "8-bit", tpdu: "040b915155210300f1000462909251341100076865" + "6c6c6f2042", from: "+15551230001", text: "hello B"},
		{name: "GSM7", tpdu: "040b915155210300f10000629092513411000" + "5c8329bfd06", from: "+15551230001", text: "Hello"},
		{name: "UCS2", tpdu: "040b915155210300f10008629092513411000400e920ac", from: "+15551230001", text: "é€"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := hex.DecodeString(tc.tpdu)
			if err != nil {
				t.Fatal(err)
			}

			from, text, err := DecodeDeliver(b)
			if err != nil || from != tc.from || text != tc.text {
				t.Fatalf("from %q text %q err %v, want %q %q", from, text, err, tc.from, tc.text)
			}
		})
	}
}

func TestGSM7RoundTrip(t *testing.T) {
	septets, ok := gsm7Septets("The quick brown fox jumps over the lazy dog @ 5$")
	if !ok {
		t.Fatal("text should be GSM7")
	}

	if got := unpackSeptets(packSeptets(septets, 0), len(septets), 0); !bytes.Equal(got, septets) {
		t.Fatalf("round trip = %v, want %v", got, septets)
	}
}
