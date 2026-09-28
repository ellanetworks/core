// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"errors"
	"testing"
)

func TestLAIWire(t *testing.T) {
	for _, tc := range []struct {
		lai  LAI
		wire []byte
	}{
		{LAI{PLMN: PLMN{MCC: "001", MNC: "01"}, LAC: 0xfffe}, []byte{0x00, 0xf1, 0x10, 0xff, 0xfe}},
		{LAI{PLMN: PLMN{MCC: "310", MNC: "410"}, LAC: 0x0102}, []byte{0x13, 0x00, 0x14, 0x01, 0x02}},
	} {
		b, err := tc.lai.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(b, tc.wire) {
			t.Fatalf("%s = % x, want % x", tc.lai, b, tc.wire)
		}

		got, err := ParseLAI(tc.wire)
		if err != nil {
			t.Fatal(err)
		}

		if got != tc.lai {
			t.Fatalf("ParseLAI(% x) = %s, want %s", tc.wire, got, tc.lai)
		}
	}
}

func TestLAIRejects(t *testing.T) {
	if _, err := ParseLAI([]byte{0x00, 0xf1, 0x10, 0xff}); err == nil {
		t.Error("4-octet LAI parsed")
	}

	if _, err := ParseLAI([]byte{0x0a, 0xf1, 0x10, 0x00, 0x01}); !errors.Is(err, ErrDigit) {
		t.Errorf("non-decimal MCC: err = %v, want ErrDigit", err)
	}

	if _, err := (LAI{PLMN: PLMN{MCC: "1", MNC: "01"}}).MarshalBinary(); err == nil {
		t.Error("invalid PLMN encoded")
	}
}
