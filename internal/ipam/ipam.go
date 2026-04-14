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

var (
	// ErrPoolExhausted is returned when no free addresses remain in a pool.
	ErrPoolExhausted = errors.New("ip pool exhausted")

	// ErrNotFound is returned by LeaseStore when no matching row exists.
	ErrNotFound = errors.New("not found")

	// ErrAlreadyExists is returned by LeaseStore on unique-constraint violation.
	ErrAlreadyExists = errors.New("already exists")
)

// Allocator assigns and releases IP addresses from a pool.
type Allocator interface {
	// Allocate assigns an IP address from pool to imsi for sessionID.
	// nodeID identifies the owning cluster node (0 in standalone).
	// It checks for existing dynamic leases (re-registration), then
	// allocates a new address.
	Allocate(ctx context.Context, pool Pool, imsi string, sessionID int, nodeID int) (netip.Addr, error)

	// Release frees the dynamic lease associated with a session.
	// Returns the released address so the caller can withdraw BGP routes.
	Release(ctx context.Context, pool Pool, sessionID int, imsi string) (netip.Addr, error)
}
