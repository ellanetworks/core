// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tpdu

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

			d, err := DecodeDeliver(b)
			if err != nil {
				t.Fatal(err)
			}

			from := d.OriginatingAddress

			text, err := d.Text()
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

func TestTextUserDataHeaderBeyondUDL(t *testing.T) {
	d, err := DecodeDeliver(mustHex(t, "440b915155001000f00000"+"62909251341100"+"01"+"00"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := d.Text(); err == nil {
		t.Fatal("no error")
	}
}

func TestDecodeDeliverRawOriginatingAddress(t *testing.T) {
	d, err := DecodeDeliver(mustHex(t, "440b915155001000f07ff6"+"62909251341100"+"03"+"027000"))
	if err != nil {
		t.Fatal(err)
	}

	if hex.EncodeToString(d.RawOriginatingAddress) != "0b915155001000f0" {
		t.Fatalf("TP-OA %x", d.RawOriginatingAddress)
	}
}

func TestSubmitAppendBinary(t *testing.T) {
	got, err := Submit{UDHI: true, Reference: 3, Destination: mustHex(t, "0b915155001000f0"), PID: 0x7f, DCS: 0xf6, UD: mustHex(t, "027100")}.AppendBinary(nil)
	if err != nil {
		t.Fatal(err)
	}

	if want := "41030b915155001000f07ff603027100"; hex.EncodeToString(got) != want {
		t.Fatalf("SMS-SUBMIT %x, want %s", got, want)
	}

	for name, m := range map[string]Submit{
		"too long":     {Destination: mustHex(t, "0b915155001000f0"), DCS: 0xf6, UD: make([]byte, MaxSubmitUserData+1)},
		"GSM 7 bit":    {Destination: mustHex(t, "0b915155001000f0"), DCS: 0x00},
		"truncated DA": {Destination: mustHex(t, "0b9151"), DCS: 0xf6},
	} {
		if _, err := m.AppendBinary(nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}

	return b
}
