// Copyright 2026 Ella Networks

// Package ipam describes IPv4 address pools used by the SMF lease path.
// Pool helpers convert between netip.Addr and offsets within a CIDR; the
// actual allocation runs under Raft in db.AllocateIPLease (which returns
// ErrPoolExhausted when no free addresses remain).
package ipam

import "errors"

// ErrPoolExhausted is returned by db.AllocateIPLease when no free address
// remains in the pool.
var ErrPoolExhausted = errors.New("ip pool exhausted")
