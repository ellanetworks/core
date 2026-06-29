// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
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

func TestDeriveNASAndKeNB(t *testing.T) {
	kasme := mustHex(t, "238e457e0f758badbca8d34bb2612c10428d426757cb5553b2b184fa64bfc549")

	enc, err := DeriveKNASEnc(kasme, 2) // 128-EEA2
	if err != nil {
		t.Fatal(err)
	}

	integ, err := DeriveKNASInt(kasme, 2) // 128-EIA2
	if err != nil {
		t.Fatal(err)
	}

	if enc == integ {
		t.Fatal("NAS ciphering and integrity keys must differ")
	}

	again, err := DeriveKNASEnc(kasme, 2)
	if err != nil || again != enc {
		t.Fatalf("derivation not deterministic: %v", err)
	}

	kenb, err := DeriveKeNB(kasme, 0)
	if err != nil {
		t.Fatal(err)
	}

	if kenb == ([32]byte{}) {
		t.Fatal("K_eNB is all zero")
	}
}

func TestDeriveNHChain(t *testing.T) {
	kasme := mustHex(t, "238e457e0f758badbca8d34bb2612c10428d426757cb5553b2b184fa64bfc549")

	kenb, err := DeriveKeNB(kasme, 0)
	if err != nil {
		t.Fatal(err)
	}

	// First NH (NCC=1) is keyed on the initial K_eNB.
	nh1, err := deriveNH(kasme, kenb[:])
	if err != nil {
		t.Fatal(err)
	}

	if nh1 == ([32]byte{}) || nh1 == kenb {
		t.Fatal("NH(1) must be non-zero and differ from K_eNB")
	}

	again, err := deriveNH(kasme, kenb[:])
	if err != nil || again != nh1 {
		t.Fatalf("NH derivation not deterministic: %v", err)
	}

	// Each subsequent NH (NCC=2..) is keyed on the previous NH; the chain advances.
	nh2, err := deriveNH(kasme, nh1[:])
	if err != nil {
		t.Fatal(err)
	}

	if nh2 == nh1 {
		t.Fatal("NH chain did not advance")
	}
}
