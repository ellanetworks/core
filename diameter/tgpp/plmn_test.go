// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tgpp

import (
	"bytes"
	"errors"
	"testing"
)

func TestPLMNEncoding(t *testing.T) {
	for _, tt := range []struct {
		plmn PLMN
		want []byte
	}{
		{PLMN{MCC: "001", MNC: "01"}, []byte{0x00, 0xf1, 0x10}},
		{PLMN{MCC: "208", MNC: "93"}, []byte{0x02, 0xf8, 0x39}},
		{PLMN{MCC: "310", MNC: "410"}, []byte{0x13, 0x00, 0x14}},
		{PLMN{MCC: "999", MNC: "999"}, []byte{0x99, 0x99, 0x99}},
	} {
		got, err := tt.plmn.MarshalBinary()
		if err != nil || !bytes.Equal(got, tt.want) {
			t.Errorf("%v encodes to %x, %v; want %x", tt.plmn, got, err, tt.want)
		}

		back, err := ParsePLMN(tt.want)
		if err != nil || back != tt.plmn {
			t.Errorf("%x decodes to %+v, %v; want %+v", tt.want, back, err, tt.plmn)
		}
	}
}

func TestPLMNInvalid(t *testing.T) {
	for _, p := range []PLMN{{}, {MCC: "01", MNC: "01"}, {MCC: "001", MNC: "1"}, {MCC: "001", MNC: "0001"}, {MCC: "00a", MNC: "01"}} {
		if _, err := p.Octets(); !errors.Is(err, ErrInvalidPLMN) {
			t.Errorf("%+v: err = %v", p, err)
		}
	}

	for _, b := range [][]byte{nil, {0x00, 0xf1}, {0x00, 0xf1, 0x10, 0x00}, {0x0a, 0xf1, 0x10}, {0x00, 0xf1, 0x1f}, {0x00, 0xe1, 0x10}} {
		if _, err := ParsePLMN(b); !errors.Is(err, ErrInvalidPLMN) {
			t.Errorf("%x: err = %v", b, err)
		}
	}
}

func TestPLMNAppendBinary(t *testing.T) {
	got, err := PLMN{MCC: "208", MNC: "93"}.AppendBinary([]byte{0xaa})
	if err != nil || !bytes.Equal(got, []byte{0xaa, 0x02, 0xf8, 0x39}) {
		t.Fatalf("AppendBinary = %x, %v", got, err)
	}

	if got, err := (PLMN{}).AppendBinary([]byte{0xaa}); !errors.Is(err, ErrInvalidPLMN) || !bytes.Equal(got, []byte{0xaa}) {
		t.Fatalf("invalid AppendBinary = %x, %v", got, err)
	}

	if s := (PLMN{MCC: "310", MNC: "410"}).String(); s != "310410" {
		t.Fatalf("String = %q", s)
	}
}

func TestParseMCCMNC(t *testing.T) {
	for s, want := range map[string]PLMN{"00101": {MCC: "001", MNC: "01"}, "310410": {MCC: "310", MNC: "410"}} {
		if got, err := ParseMCCMNC(s); err != nil || got != want {
			t.Errorf("%q = %+v, %v", s, got, err)
		}
	}

	for _, s := range []string{"", "0010", "0010100", "0010a"} {
		if _, err := ParseMCCMNC(s); !errors.Is(err, ErrInvalidPLMN) {
			t.Errorf("%q: err = %v", s, err)
		}
	}
}
