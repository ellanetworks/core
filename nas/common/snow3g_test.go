// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// The 128-EEA1 / 128-EIA1 expected outputs below were produced by a
// 3GPP-conformance-validated reference (which passes the ETSI/3GPP test sets)
// and pin this independent SNOW 3G implementation to the standard algorithm.
// They cover both directions, byte- and non-byte-aligned lengths, and the f9
// final-block zero-padding (lengths 1, 8, 9, 16).

func TestSNOW3GCipherKAT(t *testing.T) {
	cases := []struct {
		count       uint32
		bearer, dir uint8
		n           int
		want        string
	}{
		{0x12345678, 0x0a, DirectionDownlink, 20, "52e44898eb15bb5cf9286370df923af044f39bce"},
		{0x000000ff, 0x1f, DirectionUplink, 17, "55fa36b322c71678ef90b9071322715374"},
	}

	for _, tc := range cases {
		ct, err := SNOW3GCipher{}.Apply(katKey, tc.count, tc.bearer, tc.dir, seqBytes(tc.n))
		if err != nil {
			t.Fatal(err)
		}

		if hex.EncodeToString(ct) != tc.want {
			t.Fatalf("128-EEA1 dir=%d n=%d = %s, want %s", tc.dir, tc.n, hex.EncodeToString(ct), tc.want)
		}

		// Re-applying restores the plaintext (keystream XOR).
		back, _ := SNOW3GCipher{}.Apply(katKey, tc.count, tc.bearer, tc.dir, ct)
		if !bytes.Equal(back, seqBytes(tc.n)) {
			t.Fatalf("128-EEA1 round-trip mismatch for n=%d", tc.n)
		}
	}
}

func TestSNOW3GIntegrityKAT(t *testing.T) {
	cases := []struct {
		count       uint32
		bearer, dir uint8
		n           int
		want        string
	}{
		{0x12345678, 0x0a, DirectionDownlink, 20, "b2f14a68"},
		{0x000000ff, 0x1f, DirectionUplink, 7, "91230077"},
		{0x00000001, 0x00, DirectionDownlink, 1, "6573ac75"},
		{0x00000001, 0x00, DirectionDownlink, 8, "2832d7fb"},
		{0x00000001, 0x00, DirectionDownlink, 9, "15c492e0"},
		{0x00000001, 0x00, DirectionDownlink, 16, "a15c8e19"},
	}

	for _, tc := range cases {
		mac, err := SNOW3GIntegrity{}.MAC(katKey, tc.count, tc.bearer, tc.dir, seqBytes(tc.n))
		if err != nil {
			t.Fatal(err)
		}

		if hex.EncodeToString(mac[:]) != tc.want {
			t.Fatalf("128-EIA1 dir=%d n=%d = %s, want %s", tc.dir, tc.n, hex.EncodeToString(mac[:]), tc.want)
		}
	}
}
