// Copyright 2026 Ella Networks

// Package ipam implements IP Address Management for Ella Core.
// It provides an Allocator interface backed by the ip_leases database table,
// with a SequentialAllocator for IPv4 (and future IPv6 prefix delegation).
package ipam

import (
	"context"
	"errors"
	"net/netip"
)

// ErrPoolExhausted is returned when no free addresses remain in a pool.
var ErrPoolExhausted = errors.New("ip pool exhausted")

// Allocator assigns and releases IP addresses from a pool.
type Allocator interface {
	// Allocate assigns an IP address from pool to imsi for sessionID.
	// It checks for static reservations first, then existing dynamic leases
	// (re-registration), then allocates a new address.
	Allocate(ctx context.Context, pool Pool, imsi string, sessionID int) (netip.Addr, error)

	// Release frees the lease associated with a session. For dynamic leases
	// the row is deleted; for static leases the session_id is cleared.
	// Returns the released address so the caller can withdraw BGP routes.
	Release(ctx context.Context, pool Pool, sessionID int, imsi string) (netip.Addr, error)
}
