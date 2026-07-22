// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"testing"
)

func TestNASEncryptRoundTripAndMAC(t *testing.T) {
	key := [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	plain := []byte{0x7E, 0x00, 0x41, 0xDE, 0xAD, 0xBE, 0xEF}

	for _, alg := range []uint8{AlgCiphering128NEA0, AlgCiphering128NEA1, AlgCiphering128NEA2} {
		const count uint32 = 0x0000002A

		data := bytes.Clone(plain)

		if err := NASEncrypt(alg, key, count, Bearer3GPP, DirectionDownlink, data); err != nil {
			t.Fatalf("alg %d encrypt: %v", alg, err)
		}

		if alg != AlgCiphering128NEA0 && bytes.Equal(data, plain) {
			t.Errorf("alg %d did not cipher", alg)
		}

		if err := NASEncrypt(alg, key, count, Bearer3GPP, DirectionDownlink, data); err != nil {
			t.Fatalf("alg %d decrypt: %v", alg, err)
		}

		if !bytes.Equal(data, plain) {
			t.Errorf("alg %d round-trip = %x, want %x", alg, data, plain)
		}
	}

	// A wrong count changes the MAC (count is an algorithm input).
	m1, err := NASMacCalculate(AlgIntegrity128NIA2, key, 1, Bearer3GPP, DirectionUplink, plain)
	if err != nil {
		t.Fatal(err)
	}

	m2, _ := NASMacCalculate(AlgIntegrity128NIA2, key, 2, Bearer3GPP, DirectionUplink, plain)
	if bytes.Equal(m1, m2) || len(m1) != 4 {
		t.Errorf("MAC not count-dependent or wrong length: %x %x", m1, m2)
	}
}

func TestUnsupportedAlgorithms(t *testing.T) {
	key := [16]byte{}

	if _, err := NASMacCalculate(AlgIntegrity128NIA3, key, 0, Bearer3GPP, DirectionUplink, nil); err == nil {
		t.Error("NIA3 (ZUC) must be unsupported")
	}

	if err := NASEncrypt(AlgCiphering128NEA3, key, 0, Bearer3GPP, DirectionUplink, nil); err == nil {
		t.Error("NEA3 (ZUC) must be unsupported")
	}
}
