// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"testing"
)

func TestGenerateIID(t *testing.T) {
	iid, err := GenerateIID()
	if err != nil {
		t.Fatalf("GenerateIID: unexpected error: %v", err)
	}

	// Must not be all zeros (probability 2^-64, effectively impossible).
	allZero := true

	for _, b := range iid {
		if b != 0 {
			allZero = false
			break
		}
	}

	if allZero {
		t.Fatal("GenerateIID returned all zeros")
	}

	if len(iid) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(iid))
	}
}

func TestGenerateIID_Uniqueness(t *testing.T) {
	seen := make(map[[8]byte]bool)

	for i := 0; i < 1000; i++ {
		iid, err := GenerateIID()
		if err != nil {
			t.Fatalf("GenerateIID iteration %d: %v", i, err)
		}

		if seen[iid] {
			t.Fatalf("duplicate IID at iteration %d: %x", i, iid)
		}

		seen[iid] = true
	}
}
