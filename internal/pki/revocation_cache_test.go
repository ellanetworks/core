// Copyright 2026 Ella Networks

package pki_test

import (
	"math/big"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/pki"
)

func TestRevocationCache_Basics(t *testing.T) {
	c := pki.NewRevocationCache()

	if c.IsRevoked(big.NewInt(1)) {
		t.Fatal("empty cache must report not revoked")
	}

	c.Add(1)

	if !c.IsRevoked(big.NewInt(1)) {
		t.Fatal("added serial must report revoked")
	}

	c.Remove(1)

	if c.IsRevoked(big.NewInt(1)) {
		t.Fatal("removed serial must report not revoked")
	}
}

func TestRevocationCache_Replace(t *testing.T) {
	c := pki.NewRevocationCache()

	c.Add(1)
	c.Add(2)
	c.Add(3)

	c.Replace([]uint64{4, 5})

	for _, s := range []int64{1, 2, 3} {
		if c.IsRevoked(big.NewInt(s)) {
			t.Fatalf("serial %d should not be revoked after Replace", s)
		}
	}

	for _, s := range []int64{4, 5} {
		if !c.IsRevoked(big.NewInt(s)) {
			t.Fatalf("serial %d should be revoked after Replace", s)
		}
	}

	if c.Size() != 2 {
		t.Fatalf("size = %d, want 2", c.Size())
	}
}

func TestRevocationCache_NonUint64(t *testing.T) {
	c := pki.NewRevocationCache()
	c.Add(1)

	// 128-bit CA serials never overlap with the uint64 range used by leaf
	// serials, so they always read as not-revoked.
	big128 := new(big.Int).Lsh(big.NewInt(1), 100)
	if c.IsRevoked(big128) {
		t.Fatal("128-bit serial should never be revoked")
	}

	if c.IsRevoked(nil) {
		t.Fatal("nil serial should never be revoked")
	}
}

func TestRevocationCache_Concurrent(t *testing.T) {
	c := pki.NewRevocationCache()

	var wg sync.WaitGroup
	for i := uint64(0); i < 100; i++ {
		wg.Add(2)

		go func(s uint64) {
			defer wg.Done()

			c.Add(s)
		}(i)

		go func(s uint64) {
			defer wg.Done()

			_ = c.IsRevoked(new(big.Int).SetUint64(s))
		}(i)
	}

	wg.Wait()

	if c.Size() != 100 {
		t.Fatalf("Size() = %d, want 100", c.Size())
	}
}
