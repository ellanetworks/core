package server

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestDeriveOPc(t *testing.T) {
	// Test vectors from 3GPP TS 35.207 (MILENAGE algorithm test sets)
	tests := []struct {
		name        string
		K           string
		OP          string
		expectedOPc string
	}{
		{
			name:        "TS 35.207 Test Set 1",
			K:           "465b5ce8b199b49faa5f0a2ee238a6bc",
			OP:          "cdc202d5123e20f62b6d676ac72cb318",
			expectedOPc: "cd63cb71954a9f4e48a5994e37a02baf",
		},
		{
			name:        "TS 35.207 Test Set 2",
			K:           "0396eb317b6d1c36f19c1c84cd6ffd16",
			OP:          "ff53bade17df5d4e793073ce9d7579fa",
			expectedOPc: "53c15671c60a4b731c55b4a441c0bde2",
		},
		{
			name:        "TS 35.207 Test Set 3",
			K:           "fec86ba6eb707ed08905757b1bb44b8f",
			OP:          "dbc59adcb6f9a0ef735477b7fadf8374",
			expectedOPc: "1006020f0a478bf6b699f15c062e42b3",
		},
		{
			name:        "TS 35.207 Test Set 4",
			K:           "9e5944aea94b81165c82fbf9f32db751",
			OP:          "223014c5806694c007ca1eeef57f004f",
			expectedOPc: "a64a507ae1a2a98bb88eb4210135dc87",
		},
		{
			name:        "TS 35.207 Test Set 5",
			K:           "4ab1deb05ca6ceb051fc98e77d026a84",
			OP:          "2d16c5cd1fdf6b22383584e3bef2a8d8",
			expectedOPc: "dcf07cbd51855290b92a07a9891e523e",
		},
		{
			name:        "TS 35.207 Test Set 6",
			K:           "6c38a116ac280c454f59332ee35c8c4f",
			OP:          "1ba00a1a7c6700ac8c3ff3e96ad08725",
			expectedOPc: "3803ef5363b947c6aaa225e58fae3934",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			K, _ := hex.DecodeString(tt.K)
			OP, _ := hex.DecodeString(tt.OP)
			expected, _ := hex.DecodeString(tt.expectedOPc)

			result, err := deriveOPc(K, OP)
			if err != nil {
				t.Fatalf("deriveOPc() returned error: %v", err)
			}

			if !bytes.Equal(result, expected) {
				t.Errorf("deriveOPc() = %x, want %x", result, expected)
			}
		})
	}
}

func TestDeriveOPcInvalidInputs(t *testing.T) {
	valid16 := make([]byte, 16)

	if _, err := deriveOPc([]byte{0x01}, valid16); err == nil {
		t.Error("expected error for short key")
	}

	if _, err := deriveOPc(valid16, []byte{0x01}); err == nil {
		t.Error("expected error for short OP")
	}
}
