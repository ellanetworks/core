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
		got, err := tt.plmn.Encode()
		if err != nil || !bytes.Equal(got, tt.want) {
			t.Errorf("%v encodes to %x, %v; want %x", tt.plmn, got, err, tt.want)
		}

		back, err := DecodePLMN(tt.want)
		if err != nil || back != tt.plmn {
			t.Errorf("%x decodes to %+v, %v; want %+v", tt.want, back, err, tt.plmn)
		}
	}
}

func TestPLMNInvalid(t *testing.T) {
	for _, p := range []PLMN{{}, {MCC: "01", MNC: "01"}, {MCC: "001", MNC: "1"}, {MCC: "001", MNC: "0001"}, {MCC: "00a", MNC: "01"}} {
		if _, err := p.Encode(); !errors.Is(err, ErrInvalidPLMN) {
			t.Errorf("%+v: err = %v", p, err)
		}
	}

	for _, b := range [][]byte{nil, {0x00, 0xf1}, {0x00, 0xf1, 0x10, 0x00}, {0x0a, 0xf1, 0x10}, {0x00, 0xf1, 0x1f}, {0x00, 0xe1, 0x10}} {
		if _, err := DecodePLMN(b); !errors.Is(err, ErrInvalidPLMN) {
			t.Errorf("%x: err = %v", b, err)
		}
	}
}

func TestRATTypeString(t *testing.T) {
	if s := RATEUTRAN.String(); s != "EUTRAN" {
		t.Errorf("EUTRAN = %q", s)
	}

	if s := RATType(9999).String(); s != "RATType(9999)" {
		t.Errorf("unknown = %q", s)
	}
}
