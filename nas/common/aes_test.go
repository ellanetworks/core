// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// seqBytes returns n bytes 0,1,2,…, a deterministic test payload.
func seqBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}

	return b
}

// katKey is the shared 128-bit key for the AES and SNOW3G known-answer tests.
var katKey = [16]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}

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

// The 128-EEA2 / 128-EIA2 expected outputs below were produced by a
// 3GPP-conformance-validated reference (which passes the ETSI/3GPP test sets)
// and pin this implementation's counter-block and CMAC framing to the standard.
func TestAESEEA2KAT(t *testing.T) {
	cases := []struct {
		count       uint32
		bearer, dir uint8
		n           int
		want        string
	}{
		{0x12345678, 0x0a, DirectionDownlink, 20, "a5970f1784f45f8e92de42327dfa6f53518825e8"},
		{0x000000ff, 0x1f, DirectionUplink, 16, "c474524347c40af9612c05d528322bf4"},
	}

	for _, tc := range cases {
		ct, err := AESCTRCipher{}.Apply(katKey, tc.count, tc.bearer, tc.dir, seqBytes(tc.n))
		if err != nil {
			t.Fatal(err)
		}

		if hex.EncodeToString(ct) != tc.want {
			t.Fatalf("128-EEA2 dir=%d n=%d = %s, want %s", tc.dir, tc.n, hex.EncodeToString(ct), tc.want)
		}

		// Re-applying restores the plaintext (keystream XOR).
		back, _ := AESCTRCipher{}.Apply(katKey, tc.count, tc.bearer, tc.dir, ct)
		if !bytes.Equal(back, seqBytes(tc.n)) {
			t.Fatalf("128-EEA2 round-trip mismatch")
		}
	}
}

func TestAESEIA2KAT(t *testing.T) {
	cases := []struct {
		count       uint32
		bearer, dir uint8
		n           int
		want        string
	}{
		{0x12345678, 0x0a, DirectionDownlink, 20, "83f0c55c"},
		{0x000000ff, 0x1f, DirectionUplink, 7, "e94daa5c"},
	}

	for _, tc := range cases {
		mac, err := AESCMACIntegrity{}.MAC(katKey, tc.count, tc.bearer, tc.dir, seqBytes(tc.n))
		if err != nil {
			t.Fatal(err)
		}

		if hex.EncodeToString(mac[:]) != tc.want {
			t.Fatalf("128-EIA2 dir=%d n=%d = %s, want %s", tc.dir, tc.n, hex.EncodeToString(mac[:]), tc.want)
		}
	}
}
