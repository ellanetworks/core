// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"testing"
)

// TestAESCMAC_RFC4493 checks aesCMAC against the official RFC 4493 test vectors
// (AES-128 key 2b7e1516…), covering the empty, complete-block, and
// padded-last-block paths.
func TestAESCMAC_RFC4493(t *testing.T) {
	key := mustHex(t, "2b7e151628aed2a6abf7158809cf4f3c")
	msg := mustHex(t, "6bc1bee22e409f96e93d7e117393172a"+
		"ae2d8a571e03ac9c9eb76fac45af8e51"+
		"30c81c46a35ce411e5fbc1191a0a52ef"+
		"f69f2445df4f9b17ad2b417be66c3710")

	cases := []struct {
		mlen int
		want string
	}{
		{0, "bb1d6929e95937287fa37d129b756746"},
		{16, "070a16b46b4d4144f79bdd9dd04a287c"},
		{40, "dfa66747de9ae63030ca32611497c827"},
		{64, "51f0bebf7e3b9d92fc49741779363cfe"},
	}

	for _, tc := range cases {
		got, err := aesCMAC(key, msg[:tc.mlen])
		if err != nil {
			t.Fatalf("mlen %d: %v", tc.mlen, err)
		}

		if !bytes.Equal(got, mustHex(t, tc.want)) {
			t.Fatalf("mlen %d: CMAC = %x, want %s", tc.mlen, got, tc.want)
		}
	}
}

// TestEEA2Vectors runs the 128-EEA2 / 128-NEA2 test sets of TS 33.401 Annex C.1
// through the selector, which pins the algorithm identifier to the implementation
// as well as the implementation to the standard: the counter block, the bearer
// and direction packing, and the keystream XOR.
func TestEEA2Vectors(t *testing.T) {
	ciph, err := CipherFor(CipheringAES)
	if err != nil {
		t.Fatalf("CipherFor(128-NEA2): %v", err)
	}

	for _, v := range eea2Vectors {
		plain := mustHex(t, v.plain)
		want := mustHex(t, v.cipher)

		got, err := ciph.Apply(cipherKey(t, v.key), v.count, v.bearer, v.dir, plain)
		if err != nil {
			t.Fatalf("TS 33.401 %s: %v", v.set, err)
		}

		if bit := firstBitDiff(got, want, v.bits); bit >= 0 {
			t.Errorf("TS 33.401 %s: ciphertext differs at bit %d\n got %x\nwant %x", v.set, bit, got, want)
			continue
		}

		// Deciphering is the same keystream XOR, so it restores the plaintext.
		back, err := ciph.Apply(cipherKey(t, v.key), v.count, v.bearer, v.dir, got)
		if err != nil {
			t.Fatalf("TS 33.401 %s: decipher: %v", v.set, err)
		}

		if bit := firstBitDiff(back, plain, v.bits); bit >= 0 {
			t.Errorf("TS 33.401 %s: decipher differs from plaintext at bit %d", v.set, bit)
		}
	}
}

// TestEIA2Vectors runs the byte-aligned 128-EIA2 / 128-NIA2 test sets of
// TS 33.401 Annex C.2, which pin the CMAC framing and the 4-octet truncation to
// the most significant end (§B.2.3).
func TestEIA2Vectors(t *testing.T) {
	runIntegrityVectors(t, IntegrityAES, "128-EIA2", eia2Vectors)
}
