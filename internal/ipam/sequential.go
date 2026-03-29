// Copyright 2026 Ella Networks

package ipam

import (
	"context"
	"net/netip"
	"sort"
	"time"
)

// LeaseStore is the database interface required by the allocator.
// It is satisfied by *db.Database.
type LeaseStore interface {
	// GetStaticLease returns the static lease for (poolID, imsi), or ErrNotFound.
	GetStaticLease(ctx context.Context, poolID int, imsi string) (*Lease, error)

	// GetDynamicLease returns the dynamic lease for (poolID, imsi), or ErrNotFound.
	GetDynamicLease(ctx context.Context, poolID int, imsi string) (*Lease, error)

	// GetLeaseBySession returns the lease for (poolID, sessionID, imsi), or ErrNotFound.
	GetLeaseBySession(ctx context.Context, poolID int, sessionID int, imsi string) (*Lease, error)

	// ListLeaseAddressesByPool returns sorted address strings for all leases in the pool.
	ListLeaseAddressesByPool(ctx context.Context, poolID int) ([]string, error)

	// CreateLease inserts a new lease. Returns ErrAlreadyExists on unique violation.
	CreateLease(ctx context.Context, lease *Lease) error

	// UpdateLeaseSession sets the sessionID on a lease.
	UpdateLeaseSession(ctx context.Context, leaseID int, sessionID int) error

	// DeleteDynamicLease deletes a dynamic lease by ID (no-op for static).
	DeleteDynamicLease(ctx context.Context, leaseID int) error

	// ClearLeaseSession sets sessionID to NULL on a lease.
	ClearLeaseSession(ctx context.Context, leaseID int) error
}

// Lease mirrors db.IPLease but lives in the ipam package to avoid an import
// cycle. The db package satisfies LeaseStore via adapter methods.
type Lease struct {
	ID        int
	PoolID    int
	Address   string
	IMSI      string
	SessionID *int
	Type      string
	CreatedAt int64
}

// ErrNotFound is the sentinel returned by LeaseStore when no matching row exists.
// It is compared by value, so the db package's ErrNotFound must be the same.
var ErrNotFound = errNotFound{}

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }

// IsNotFound reports whether err represents a "not found" condition.
// It matches both ipam.ErrNotFound and any error whose Error() is "not found"
// (which includes db.ErrNotFound).
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	return err.Error() == "not found"
}

// IsAlreadyExists reports whether err represents a unique-constraint violation.
func IsAlreadyExists(err error) bool {
	if err == nil {
		return false
	}

	return err.Error() == "already exists"
}

// SequentialAllocator implements the merge-scan allocation algorithm for IPv4.
// It walks the pool sequentially to find the first free address.
type SequentialAllocator struct {
	store LeaseStore
}

// NewSequentialAllocator creates an allocator backed by the given store.
func NewSequentialAllocator(store LeaseStore) *SequentialAllocator {
	return &SequentialAllocator{store: store}
}

// Allocate assigns an address from pool to imsi for the given sessionID.
//
// Algorithm:
//  0. Check for a static reservation — if found, attach the session and return.
//  1. Check for an existing dynamic lease (re-registration) — reuse it.
//  2. Fetch all allocated addresses as offsets (one query, sorted).
//  3. Merge-scan: walk offsets [FirstUsable, FirstUsable+Size), skipping allocated.
//  4. INSERT the first free address. On unique violation (race), retry next.
func (a *SequentialAllocator) Allocate(ctx context.Context, pool Pool, imsi string, sessionID int) (netip.Addr, error) {
	// Step 0: static reservation.
	staticLease, err := a.store.GetStaticLease(ctx, pool.ID, imsi)
	if err == nil {
		if err := a.store.UpdateLeaseSession(ctx, staticLease.ID, sessionID); err != nil {
			return netip.Addr{}, err
		}

		addr, parseErr := netip.ParseAddr(staticLease.Address)
		if parseErr != nil {
			return netip.Addr{}, parseErr
		}

		return addr, nil
	} else if !IsNotFound(err) {
		return netip.Addr{}, err
	}

	// Step 1: existing dynamic lease (re-registration).
	existing, err := a.store.GetDynamicLease(ctx, pool.ID, imsi)
	if err == nil {
		if err := a.store.UpdateLeaseSession(ctx, existing.ID, sessionID); err != nil {
			return netip.Addr{}, err
		}

		addr, parseErr := netip.ParseAddr(existing.Address)
		if parseErr != nil {
			return netip.Addr{}, parseErr
		}

		return addr, nil
	} else if !IsNotFound(err) {
		return netip.Addr{}, err
	}

	// Step 2: fetch all allocated addresses for this pool.
	addresses, err := a.store.ListLeaseAddressesByPool(ctx, pool.ID)
	if err != nil {
		return netip.Addr{}, err
	}

	// Convert addresses to offsets and sort them.
	allocated := make([]int, 0, len(addresses))
	for _, addrStr := range addresses {
		addr, parseErr := netip.ParseAddr(addrStr)
		if parseErr != nil {
			continue
		}

		offset := pool.OffsetOf(addr)
		if offset >= 0 {
			allocated = append(allocated, offset)
		}
	}

	sort.Ints(allocated)

	// Step 3: merge-scan to find the first free offset.
	poolSize := pool.Size()
	firstUsable := pool.FirstUsable()
	allocIdx := 0
	now := time.Now().Unix()

	for offset := firstUsable; offset < firstUsable+poolSize; offset++ {
		// Advance past allocated offsets that are below current offset.
		for allocIdx < len(allocated) && allocated[allocIdx] < offset {
			allocIdx++
		}

		if allocIdx < len(allocated) && allocated[allocIdx] == offset {
			continue // taken
		}

		// Found a free offset — try to claim it.
		addr := pool.AddressAtOffset(offset)

		lease := &Lease{
			PoolID:    pool.ID,
			Address:   addr.String(),
			IMSI:      imsi,
			SessionID: &sessionID,
			Type:      "dynamic",
			CreatedAt: now,
		}

		err := a.store.CreateLease(ctx, lease)
		if err == nil {
			return addr, nil
		}

		if IsAlreadyExists(err) {
			continue // race — another goroutine grabbed it
		}

		return netip.Addr{}, err
	}

	return netip.Addr{}, ErrPoolExhausted
}

// Release frees the lease for (pool, sessionID, imsi). For dynamic leases
// the row is deleted; for static leases the session_id is cleared.
// Returns the released address.
func (a *SequentialAllocator) Release(ctx context.Context, pool Pool, sessionID int, imsi string) (netip.Addr, error) {
	lease, err := a.store.GetLeaseBySession(ctx, pool.ID, sessionID, imsi)
	if err != nil {
		return netip.Addr{}, err
	}

	addr, parseErr := netip.ParseAddr(lease.Address)
	if parseErr != nil {
		return netip.Addr{}, parseErr
	}

	if lease.Type == "static" {
		// Static leases are never deleted — clear the session instead.
		if err := a.store.ClearLeaseSession(ctx, lease.ID); err != nil {
			return netip.Addr{}, err
		}
	} else {
		// Dynamic leases are deleted.
		if err := a.store.DeleteDynamicLease(ctx, lease.ID); err != nil {
			return netip.Addr{}, err
		}
	}

	return addr, nil
}
