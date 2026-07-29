// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

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

// katKey is the 128-bit key of the 128-EEA1 known-answer test below.
var katKey = [16]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}

// TestEIA1Vectors runs the byte-aligned 128-EIA1 / 128-NIA1 test sets of
// TS 33.401 Annex C.4 through the selector. They exercise the SNOW 3G key and IV
// loading, the keystream generator, and the f9 accumulation and truncation.
func TestEIA1Vectors(t *testing.T) {
	runIntegrityVectors(t, IntegritySNOW3G, "128-EIA1", eia1Vectors)
}

// TestSNOW3GCipherKAT pins 128-EEA1 / 128-NEA1. TS 33.401 Annex C.3 publishes no
// data of its own for it — it defers to the UEA2 sets of TS 35.216, which the
// annex does not reproduce — so these expected outputs come from a
// 3GPP-conformance-validated reference implementation rather than from a citable
// clause. What they add over the f9 vectors above, which cover the same SNOW 3G
// engine, is the f8 IV construction and the keystream XOR.
func TestSNOW3GCipherKAT(t *testing.T) {
	cases := []struct {
		count  uint32
		bearer Bearer
		dir    Direction
		n      int
		want   string
	}{
		{0x12345678, 0x0a, DirectionDownlink, 20, "52e44898eb15bb5cf9286370df923af044f39bce"},
		{0x000000ff, 0x1f, DirectionUplink, 17, "55fa36b322c71678ef90b9071322715374"},
	}

	ciph, err := CipherFor(CipheringSNOW3G)
	if err != nil {
		t.Fatalf("CipherFor(128-NEA1): %v", err)
	}

	for _, tc := range cases {
		ct, err := ciph.Apply(katKey, tc.count, tc.bearer, tc.dir, seqBytes(tc.n))
		if err != nil {
			t.Fatal(err)
		}

		if hex.EncodeToString(ct) != tc.want {
			t.Fatalf("128-EEA1 dir=%d n=%d = %s, want %s", tc.dir, tc.n, hex.EncodeToString(ct), tc.want)
		}

		// Re-applying restores the plaintext (keystream XOR).
		back, _ := ciph.Apply(katKey, tc.count, tc.bearer, tc.dir, ct)
		if !bytes.Equal(back, seqBytes(tc.n)) {
			t.Fatalf("128-EEA1 round-trip mismatch for n=%d", tc.n)
		}
	}
}
