// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sqn_test

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/milenage"
	"github.com/ellanetworks/core/internal/sqn"
)

const (
	testK   = "465b5ce8b199b49faa5f0a2ee238a6bc"
	testOPc = "cd63cb71954a9f4e48a5994e37a02baf"
)

func TestNextIMSUsesTheIMSIndexWithTheNextSEQ(t *testing.T) {
	cases := []struct {
		current, want string
	}{
		{"000000000000", "00000000003f"},
		{"000000000020", "00000000005f"},
		{"00000000003f", "00000000005f"},
		{"00000000043e", "00000000045f"},
		{"ffffffffffff", "00000000001f"},
	}

	for _, tc := range cases {
		got, err := sqn.NextIMS(tc.current)
		if err != nil {
			t.Fatalf("NextIMS(%s): %v", tc.current, err)
		}

		if got != tc.want {
			t.Fatalf("NextIMS(%s) = %s, want %s", tc.current, got, tc.want)
		}
	}
}

func TestNextKeepsTheIndexOutsideTheIMSRange(t *testing.T) {
	cases := []struct {
		current, want string
	}{
		{"000000000020", "000000000040"},
		{"000000000025", "000000000045"},
		{"00000000003f", "000000000060"},
	}

	for _, tc := range cases {
		got, err := sqn.Next(tc.current, testOPc, testK, "", "")
		if err != nil {
			t.Fatalf("Next(%s): %v", tc.current, err)
		}

		if got != tc.want {
			t.Fatalf("Next(%s) = %s, want %s", tc.current, got, tc.want)
		}
	}
}

func testRAND() []byte {
	rand := make([]byte, 16)

	for i := range rand {
		rand[i] = byte(i)
	}

	return rand
}

func testAUTS(t *testing.T, rand []byte, sqnMSHex string) string {
	t.Helper()

	k, _ := hex.DecodeString(testK)
	opc, _ := hex.DecodeString(testOPc)
	sqnMS, _ := hex.DecodeString(sqnMSHex)

	akStar := make([]byte, 6)
	if err := milenage.F2345(opc, k, rand, nil, nil, nil, nil, akStar); err != nil {
		t.Fatalf("f5*: %v", err)
	}

	macA, macS := make([]byte, 8), make([]byte, 8)
	if err := milenage.F1(opc, k, rand, sqnMS, []byte{0, 0}, macA, macS); err != nil {
		t.Fatalf("f1*: %v", err)
	}

	auts := make([]byte, 0, 14)
	for i := range sqnMS {
		auts = append(auts, sqnMS[i]^akStar[i])
	}

	return hex.EncodeToString(append(auts, macS...))
}

func TestNextResyncSkipsTheIMSIndex(t *testing.T) {
	rand := testRAND()

	got, err := sqn.Next("000000000020", testOPc, testK, testAUTS(t, rand, "00000000005e"), hex.EncodeToString(rand))
	if err != nil {
		t.Fatalf("Next with AUTS: %v", err)
	}

	if got != "000000000080" {
		t.Fatalf("Next with AUTS = %s, want 000000000080", got)
	}
}

func TestNextIMSResync(t *testing.T) {
	cases := []struct {
		name, current, sqnMS, want string
	}{
		{"the USIM is ahead", "000000000020", "00000000105e", "00000000107f"},
		{"the USIM is ahead in the IMS index", "000000000020", "00000000107f", "00000000109f"},
		{"the network is ahead", "00000000209f", "00000000105e", "0000000020bf"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rand := testRAND()

			got, err := sqn.NextIMSResync(tc.current, testOPc, testK, testAUTS(t, rand, tc.sqnMS), hex.EncodeToString(rand))
			if err != nil {
				t.Fatalf("NextIMSResync: %v", err)
			}

			if got != tc.want {
				t.Fatalf("NextIMSResync = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestNextIMSResyncRejectsABadMACS(t *testing.T) {
	rand := testRAND()

	auts, _ := hex.DecodeString(testAUTS(t, rand, "00000000105e"))
	auts[13] ^= 0x01

	if _, err := sqn.NextIMSResync("000000000020", testOPc, testK, hex.EncodeToString(auts), hex.EncodeToString(rand)); err == nil {
		t.Fatal("NextIMSResync accepted an AUTS with a bad MAC-S")
	}
}
