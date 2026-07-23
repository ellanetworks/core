// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package milenage_test

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/util/milenage"
)

// Test Set 1 from 3GPP TS 35.207 / TS 35.208 §4.3.
const (
	ts1OPc  = "cd63cb71954a9f4e48a5994e37a02baf"
	ts1K    = "465b5ce8b199b49faa5f0a2ee238a6bc"
	ts1RAND = "23553cbe9637a89d218ae64dae47bf35"
	ts1SQN  = "ff9bb4d0b607"
	ts1AMF  = "b9b9"
	ts1MACA = "4a9ffac354dfafb3"                 // f1
	ts1RES  = "a54211d5e3ba50bf"                 // f2
	ts1CK   = "b40ba9a3c58b2a05bbf0d987b21bf8cb" // f3
	ts1IK   = "f769bcd751044604127672711c6d3441" // f4
	ts1AK   = "aa689c648370"                     // f5
	ts1AKS  = "451e8beca43b"                     // f5*
)

func mustDecode(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}

	return b
}

func xorBytes(a, b []byte) []byte {
	out := make([]byte, len(a))
	for i := range a {
		out[i] = a[i] ^ b[i]
	}

	return out
}

func TestGenerateKeysWithAUTN(t *testing.T) {
	opc := mustDecode(t, ts1OPc)
	k := mustDecode(t, ts1K)
	rand := mustDecode(t, ts1RAND)
	sqn := mustDecode(t, ts1SQN)
	ak := mustDecode(t, ts1AK)

	// AUTN = (SQN XOR AK) || AMF || MAC-A.
	autn := append(xorBytes(sqn, ak), mustDecode(t, ts1AMF)...)
	autn = append(autn, mustDecode(t, ts1MACA)...)

	gotSQN, gotAK, gotIK, gotCK, gotRES, err := milenage.GenerateKeysWithAUTN(opc, k, rand, autn)
	if err != nil {
		t.Fatalf("GenerateKeysWithAUTN: %v", err)
	}

	for _, c := range []struct {
		name string
		got  []byte
		want string
	}{
		{"SQN", gotSQN, ts1SQN},
		{"AK", gotAK, ts1AK},
		{"IK", gotIK, ts1IK},
		{"CK", gotCK, ts1CK},
		{"RES", gotRES, ts1RES},
	} {
		if got := hex.EncodeToString(c.got); got != c.want {
			t.Errorf("%s = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestGenerateKeysWithAUTNMACFailure(t *testing.T) {
	opc := mustDecode(t, ts1OPc)
	k := mustDecode(t, ts1K)
	rand := mustDecode(t, ts1RAND)
	sqn := mustDecode(t, ts1SQN)
	ak := mustDecode(t, ts1AK)

	autn := append(xorBytes(sqn, ak), mustDecode(t, ts1AMF)...)
	autn = append(autn, mustDecode(t, ts1MACA)...)
	autn[len(autn)-1] ^= 0xff // corrupt MAC-A

	if _, _, _, _, _, err := milenage.GenerateKeysWithAUTN(opc, k, rand, autn); err == nil {
		t.Fatal("expected MAC failure, got nil")
	}
}

func TestGenerateAUTS(t *testing.T) {
	opc := mustDecode(t, ts1OPc)
	k := mustDecode(t, ts1K)
	rand := mustDecode(t, ts1RAND)
	sqnms := mustDecode(t, ts1SQN)

	auts, err := milenage.GenerateAUTS(opc, k, rand, sqnms)
	if err != nil {
		t.Fatalf("GenerateAUTS: %v", err)
	}

	// AUTS = (SQNms XOR AK*) || MAC-S; MAC-S uses the all-zero resync AMF
	// (TS 33.102 §6.3.3), so only the AK*-masked prefix has a published vector.
	if len(auts) != 14 {
		t.Fatalf("AUTS length = %d, want 14", len(auts))
	}

	wantPrefix := hex.EncodeToString(xorBytes(sqnms, mustDecode(t, ts1AKS)))
	if got := hex.EncodeToString(auts[:6]); got != wantPrefix {
		t.Errorf("AUTS SQNms XOR AK* = %s, want %s", got, wantPrefix)
	}
}

func TestValidatesArgLengths(t *testing.T) {
	valid := make([]byte, 16)
	sqn := make([]byte, 6)

	if _, _, _, _, _, err := milenage.GenerateKeysWithAUTN(valid[:4], valid, valid, valid); err == nil {
		t.Error("expected error for short OPc")
	}

	if _, err := milenage.GenerateAUTS(valid, valid, valid, sqn[:2]); err == nil {
		t.Error("expected error for short SQNms")
	}
}
