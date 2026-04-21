// Copyright 2026 Ella Networks

package pki

import (
	"math/big"
	"sync"
)

// RevocationCache is an in-memory set of revoked leaf serial numbers,
// consulted per handshake by the cluster listener.
//
// The cache is fed by the Raft observer that watches the
// cluster_revoked_certs table; on every applied command that inserts a
// revocation row, the observer calls Add; on every delete (tidy worker),
// it calls Remove; on leader change or snapshot restore, the observer
// rebuilds the cache via Replace.
type RevocationCache struct {
	mu      sync.RWMutex
	serials map[uint64]struct{}
}

// NewRevocationCache returns an empty cache.
func NewRevocationCache() *RevocationCache {
	return &RevocationCache{serials: make(map[uint64]struct{})}
}

// Add marks serial as revoked. Idempotent.
func (c *RevocationCache) Add(serial uint64) {
	c.mu.Lock()
	c.serials[serial] = struct{}{}
	c.mu.Unlock()
}

// Remove clears serial from the revocation set. Idempotent.
func (c *RevocationCache) Remove(serial uint64) {
	c.mu.Lock()
	delete(c.serials, serial)
	c.mu.Unlock()
}

// Replace swaps the entire revocation set atomically.
func (c *RevocationCache) Replace(serials []uint64) {
	next := make(map[uint64]struct{}, len(serials))
	for _, s := range serials {
		next[s] = struct{}{}
	}

	c.mu.Lock()
	c.serials = next
	c.mu.Unlock()
}

// IsRevoked reports whether serial is in the cache.
func (c *RevocationCache) IsRevoked(serial *big.Int) bool {
	if serial == nil || !serial.IsUint64() {
		// Serials allocated through the replicated counter are uint64;
		// anything else (e.g. random CA serials from the issuer itself)
		// is never in the revocation set.
		return false
	}

	c.mu.RLock()
	_, ok := c.serials[serial.Uint64()]
	c.mu.RUnlock()

	return ok
}

// Size returns the current number of revoked serials. Used by metrics.
func (c *RevocationCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.serials)
}
