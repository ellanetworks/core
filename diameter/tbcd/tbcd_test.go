// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tbcd

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"5155210300f1", "15551230001", false},
		{"21436587", "12345678", false},
		{"f1", "1", false},
		{"ba", "*#", false},
		{"dc", "ab", false},
		{"fe", "c", false},
		{"1f21", "", true},
		{"", "", true},
		{"1f", "", true},
		{"ff", "", true},
	}

	for _, tt := range tests {
		b, _ := hex.DecodeString(tt.in)

		got, err := Decode(b)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("Decode(%s) = %q, %v", tt.in, got, err)
		}
	}
}

func TestEncode(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"15551230001", "5155210300f1", false},
		{"12345678", "21436587", false},
		{"1", "f1", false},
		{"*#", "ba", false},
		{"AbC", "dcfe", false},
		{"", "", true},
		{"12d", "", true},
		{"+1", "", true},
	}

	for _, tt := range tests {
		got, err := Encode(tt.in)
		want, _ := hex.DecodeString(tt.want)

		if (err != nil) != tt.wantErr || !bytes.Equal(got, want) {
			t.Errorf("Encode(%q) = %x, %v", tt.in, got, err)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	for _, digits := range []string{"1", "12", "15551230001", "001010000000001", "*#abc0"} {
		b, err := Encode(digits)
		if err != nil {
			t.Fatal(err)
		}

		got, err := Decode(b)
		if err != nil || got != digits {
			t.Fatalf("round trip %q = %q, %v", digits, got, err)
		}
	}
}

func FuzzDecode(f *testing.F) {
	for _, seed := range [][]byte{{0x51, 0x55, 0x21, 0x03, 0x00, 0xf1}, {0xf1}, {0xff}, {0x1f, 0x21}, {}} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, b []byte) {
		digits, err := Decode(b)
		if err != nil {
			return
		}

		again, err := Encode(digits)
		if err != nil || !bytes.Equal(again, b) {
			t.Fatalf("Decode(%x) = %q re-encodes to %x, %v", b, digits, again, err)
		}
	})
}

func FuzzEncode(f *testing.F) {
	for _, seed := range []string{"15551230001", "1", "*#abc", "", "12d"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, digits string) {
		b, err := Encode(digits)
		if err != nil {
			return
		}

		got, err := Decode(b)
		if err != nil || got != strings.ToLower(digits) {
			t.Fatalf("Encode(%q) = %x decodes to %q, %v", digits, b, got, err)
		}
	})
}
