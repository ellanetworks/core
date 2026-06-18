// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package udm

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}

	return b
}

// TestDeriveKASME checks K_ASME (TS 33.401 §A.2).
func TestDeriveKASME(t *testing.T) {
	ck := mustHex(t, "b40ba9a3c58b2a05bbf0d987b21bf8cb")
	ik := mustHex(t, "f769bcd751044604127672711c6d3441")
	plmn := mustHex(t, "024830")
	sqn := mustHex(t, "fd8eef40df7d")
	ak := mustHex(t, "aa689c648370")
	want := mustHex(t, "238e457e0f758badbca8d34bb2612c10428d426757cb5553b2b184fa64bfc549")

	sqnXorAK := make([]byte, 6)
	for i := range sqnXorAK {
		sqnXorAK[i] = sqn[i] ^ ak[i]
	}

	got, err := deriveKASME(ck, ik, plmn, sqnXorAK)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("K_ASME =\n %x\nwant\n %x", got, want)
	}
}
