// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package udm

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// buildAUTS constructs a valid AUTS = (SQN_MS ⊕ AK*) ‖ MAC-S for the given
// credentials and SQN, as a UE would on a synch failure.
func buildAUTS(t *testing.T, opc, k, rand, sqnMS []byte) []byte {
	t.Helper()

	ak := make([]byte, 6)
	if err := F2345(opc, k, rand, nil, nil, nil, nil, ak); err != nil {
		t.Fatal(err)
	}

	conc := make([]byte, 6)
	for i := range conc {
		conc[i] = sqnMS[i] ^ ak[i]
	}

	amf, _ := hex.DecodeString("0000")
	macS := make([]byte, 8)

	if err := F1(opc, k, rand, sqnMS, amf, nil, macS); err != nil {
		t.Fatal(err)
	}

	return append(conc, macS...)
}

func TestResyncSQNExport(t *testing.T) {
	k, _ := hex.DecodeString("465b5ce8b199b49faa5f0a2ee238a6bc")
	rand, _ := hex.DecodeString("23553cbe9637a89d218ae64dae47bf35")
	op, _ := hex.DecodeString("cdc202d5123e20f62b6d676ac72cb318")

	opc, err := computeOPc(k, op)
	if err != nil {
		t.Fatal(err)
	}

	sqnMS, _ := hex.DecodeString("000000000021") // SEQ with a non-zero IND
	auts := buildAUTS(t, opc, k, rand, sqnMS)

	next, err := ResyncSQN(opc, k, auts, rand)
	if err != nil {
		t.Fatalf("ResyncSQN: %v", err)
	}

	// Next SQN is the recovered SQN advanced by one SEQ step (IndStep = 32).
	want, _ := hex.DecodeString("000000000041")
	if !bytes.Equal(next, want) {
		t.Fatalf("next SQN = %x, want %x", next, want)
	}

	t.Run("tampered AUTS rejected", func(t *testing.T) {
		bad := append([]byte(nil), auts...)
		bad[len(bad)-1] ^= 0xff // corrupt MAC-S

		if _, err := ResyncSQN(opc, k, bad, rand); err == nil {
			t.Fatal("expected MAC-S verification failure")
		}
	})
}
