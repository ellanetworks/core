// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"crypto/aes"
	"encoding/hex"
	"testing"
)

// computeOPc computes OPc = AES_K(OP) XOR OP per TS 33.102.
func computeOPc(k, op []byte) ([]byte, error) {
	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}

	opc := make([]byte, block.BlockSize())
	block.Encrypt(opc, op)

	for i := range 16 {
		opc[i] ^= op[i]
	}

	return opc, nil
}

func TestIncrementSQN(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "basic increment",
			input: "000000000000",
			want:  "000000000001",
		},
		{
			name:  "mid-range",
			input: "000000000100",
			want:  "000000000101",
		},
		{
			name:  "wraps at sqnMax minus one",
			input: "07fffffffffe",
			want:  "000000000000",
		},
		{
			name:  "wraps past sqnMax to one",
			input: "07ffffffffff",
			want:  "000000000001",
		},
		{
			name:    "invalid hex",
			input:   "not-hex",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := incrementSQN(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf("incrementSQN(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStrictHex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		n     int
		want  string
	}{
		{
			name:  "pad short string",
			input: "ab",
			n:     6,
			want:  "0000ab",
		},
		{
			name:  "exact length",
			input: "abcdef",
			n:     6,
			want:  "abcdef",
		},
		{
			name:  "truncate long string",
			input: "00aabbccddee",
			n:     6,
			want:  "ccddee",
		},
		{
			name:  "empty string pad",
			input: "",
			n:     4,
			want:  "0000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strictHex(tt.input, tt.n)
			if got != tt.want {
				t.Fatalf("strictHex(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
			}
		})
	}
}

func TestResyncSQN(t *testing.T) {
	// Use known Milenage test vectors from TS 33.102.
	// Test set 1 values:
	k, _ := hex.DecodeString("465b5ce8b199b49faa5f0a2ee238a6bc")
	rand, _ := hex.DecodeString("23553cbe9637a89d218ae64dae47bf35")

	// Compute OPc from K and OP (test set 1).
	op, _ := hex.DecodeString("cdc202d5123e20f62b6d676ac72cb318")

	opc, err := computeOPc(k, op)
	if err != nil {
		t.Fatalf("computeOPc failed: %v", err)
	}

	// We need to generate valid AUTS for this test. AUTS = (SQN_MS XOR AK*) || MAC-S
	// For this, we first compute AK using F2345 to XOR with a known SQN_MS, then compute
	// MAC-S with F1 using AMF=0000.
	sqnMs, _ := hex.DecodeString("000000000001")

	AK := make([]byte, 6)

	err = F2345(opc, k, rand, nil, nil, nil, nil, AK)
	if err != nil {
		t.Fatalf("F2345 failed: %v", err)
	}

	concSQN := make([]byte, 6)
	for i := range 6 {
		concSQN[i] = sqnMs[i] ^ AK[i]
	}

	amfResync, _ := hex.DecodeString("0000")
	macS := make([]byte, 8)

	err = F1(opc, k, rand, sqnMs, amfResync, nil, macS)
	if err != nil {
		t.Fatalf("F1 failed: %v", err)
	}

	auts := append(concSQN, macS...)

	recovered, next, err := resyncSQN(opc, k, auts, rand)
	if err != nil {
		t.Fatalf("resyncSQN failed: %v", err)
	}

	if hex.EncodeToString(recovered) != hex.EncodeToString(sqnMs) {
		t.Fatalf("expected recovered SQN %x, got %x", sqnMs, recovered)
	}

	// Next SQN should be sqnMs + 1
	expectedNext := "000000000002"
	if next != expectedNext {
		t.Fatalf("expected next SQN %s, got %s", expectedNext, next)
	}
}

func TestResyncSQN_BadMAC(t *testing.T) {
	k, _ := hex.DecodeString("465b5ce8b199b49faa5f0a2ee238a6bc")
	rand, _ := hex.DecodeString("23553cbe9637a89d218ae64dae47bf35")
	op, _ := hex.DecodeString("cdc202d5123e20f62b6d676ac72cb318")
	opc, _ := computeOPc(k, op)

	// Create AUTS with tampered MAC-S (all zeros).
	auts, _ := hex.DecodeString("aabbccddeeff0000000000000000")

	_, _, err := resyncSQN(opc, k, auts, rand)
	if err == nil {
		t.Fatal("expected error for bad MAC-S")
	}
}

func TestAucSQN(t *testing.T) {
	k, _ := hex.DecodeString("465b5ce8b199b49faa5f0a2ee238a6bc")
	rand, _ := hex.DecodeString("23553cbe9637a89d218ae64dae47bf35")
	op, _ := hex.DecodeString("cdc202d5123e20f62b6d676ac72cb318")
	opc, _ := computeOPc(k, op)

	sqnMs, _ := hex.DecodeString("000000000010")

	AK := make([]byte, 6)
	_ = F2345(opc, k, rand, nil, nil, nil, nil, AK)

	concSQN := make([]byte, 6)
	for i := range 6 {
		concSQN[i] = sqnMs[i] ^ AK[i]
	}

	amfBytes, _ := hex.DecodeString("0000")
	macS := make([]byte, 8)
	_ = F1(opc, k, rand, sqnMs, amfBytes, nil, macS)

	auts := append(concSQN, macS...)

	recoveredSQN, recoveredMacS, err := aucSQN(opc, k, auts, rand)
	if err != nil {
		t.Fatalf("aucSQN failed: %v", err)
	}

	if hex.EncodeToString(recoveredSQN) != hex.EncodeToString(sqnMs) {
		t.Fatalf("expected SQN %x, got %x", sqnMs, recoveredSQN)
	}

	if hex.EncodeToString(recoveredMacS) != hex.EncodeToString(macS) {
		t.Fatalf("expected MAC-S %x, got %x", macS, recoveredMacS)
	}
}
