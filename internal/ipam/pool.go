// Copyright 2026 Ella Networks

package ipam

import (
	"fmt"
	"math/big"
	"net/netip"
)

// Pool describes an IP address pool backed by a data network's CIDR.
// It provides methods to compute usable address ranges and convert between
// offsets and addresses. The offset is relative to the network base address
// (e.g. for 10.0.0.0/24, offset 1 = 10.0.0.1).
//
// PrefixLen controls the allocation granularity:
//   - For IPv4 individual addresses: PrefixLen = 32 (default via NewPool).
//   - For IPv6 /64 prefix delegation: PrefixLen = 64 (set via NewPool6).
//
// When PrefixLen < addrBits, each offset selects a sub-prefix of the pool
// rather than an individual address. For example, a /60 pool with PrefixLen=64
// has 16 allocatable /64 prefixes (offsets 0–15).
type Pool struct {
	// ID is the data_networks.id primary key (UUID), used as pool_id in ip_leases.
	ID string

	// Prefix is the parsed CIDR (e.g. "10.45.0.0/22" → 10.45.0.0/22).
	Prefix netip.Prefix

	// PrefixLen is the allocation granularity in bits. For IPv4 individual
	// address allocation this equals 32 (the address length). For IPv6 /64
	// prefix delegation this equals 64. Must satisfy:
	//   Prefix.Bits() <= PrefixLen <= Prefix.Addr().BitLen()
	PrefixLen int

	// IPVersion identifies the pool type: "ipv4" or "ipv6". Used to
	// distinguish IPv4 and IPv6 leases in the ip_leases table.
	IPVersion string
}

// NewPool creates a Pool from a data network ID and CIDR string.
func NewPool(id string, cidr string) (Pool, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return Pool{}, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}

	// Normalize to the masked form (e.g. 10.45.0.1/22 → 10.45.0.0/22).
	prefix = netip.PrefixFrom(prefix.Masked().Addr(), prefix.Bits())

	return Pool{ID: id, Prefix: prefix, PrefixLen: prefix.Addr().BitLen(), IPVersion: "ipv4"}, nil
}

// NewPool6 creates an IPv6 Pool with a specific allocation prefix length.
// For /64 prefix delegation, pass prefixLen = 64. The pool's CIDR must be
// an IPv6 prefix whose length is <= prefixLen (e.g. /60 pool with prefixLen=64
// yields 16 allocatable /64 prefixes).
func NewPool6(id string, cidr string, prefixLen int) (Pool, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return Pool{}, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}

	if !prefix.Addr().Is6() {
		return Pool{}, fmt.Errorf("NewPool6 requires an IPv6 CIDR, got %q", cidr)
	}

	prefix = netip.PrefixFrom(prefix.Masked().Addr(), prefix.Bits())

	if prefixLen < prefix.Bits() || prefixLen > 128 {
		return Pool{}, fmt.Errorf("prefixLen %d out of range [%d, 128]", prefixLen, prefix.Bits())
	}

	return Pool{ID: id, Prefix: prefix, PrefixLen: prefixLen, IPVersion: "ipv6"}, nil
}

// FirstUsable returns the offset of the first usable host address.
// For IPv4 this is 1 (skipping the network address).
// For IPv6 this is 0 (no network/broadcast concept for individual addresses
// or delegated prefixes).
func (p Pool) FirstUsable() int {
	if p.Prefix.Addr().Is4() {
		return 1
	}

	return 0
}

// Size returns the number of allocatable units in the pool.
//
// For IPv4 individual addresses (PrefixLen=32): total addresses minus network
// and broadcast (e.g. /24 → 254).
//
// For IPv6 prefix delegation (PrefixLen=64): the number of delegatable
// prefixes (e.g. /60 pool → 16 /64 prefixes).
func (p Pool) Size() int {
	allocBits := p.PrefixLen - p.Prefix.Bits()

	if allocBits >= 31 {
		// Very large pool — cap at max int to avoid overflow.
		return int(^uint(0) >> 1)
	}

	total := 1 << allocBits

	if p.Prefix.Addr().Is4() && p.PrefixLen == 32 {
		// IPv4 individual address allocation: subtract network + broadcast.
		return total - 2
	}

	return total
}

// AddressAtOffset returns the address at the given offset from the base
// network address. For individual address pools (PrefixLen == addrBits),
// offset 0 = base address, offset 1 = base+1, etc.
//
// For prefix delegation pools (PrefixLen < addrBits), offset selects
// the sub-prefix within the pool. For a /60 IPv6 pool with PrefixLen=64,
// offset 0 = first /64, offset 1 = second /64, etc. The returned address
// is the base of the delegated prefix (lower bits = 0).
//
// The caller is responsible for ensuring offset is within
// [FirstUsable, FirstUsable+Size).
func (p Pool) AddressAtOffset(offset int) netip.Addr {
	base := p.Prefix.Addr()
	addrBits := base.BitLen()

	b := base.As16()

	if p.PrefixLen == addrBits {
		// Fast path: individual address allocation (no shift needed).
		addIntToIP(b[:], offset)
	} else {
		// Shifted allocation: place offset in bits [Prefix.Bits(), PrefixLen).
		// shifted = offset << (addrBits - PrefixLen), then add to base.
		// Uses big.Int because the shift can exceed 63 bits for IPv6.
		baseInt := new(big.Int).SetBytes(b[:])
		shifted := new(big.Int).Lsh(big.NewInt(int64(offset)), uint(addrBits-p.PrefixLen))
		baseInt.Add(baseInt, shifted)
		baseInt.FillBytes(b[:])
	}

	if base.Is4() {
		return netip.AddrFrom4([4]byte(b[12:16]))
	}

	return netip.AddrFrom16(b)
}

// OffsetOf returns the offset of addr relative to the pool's base address.
// Returns -1 if addr is not within the pool's prefix.
//
// For prefix delegation pools, the address should be the base of the
// delegated prefix (lower bits = 0). The method extracts bits
// [Prefix.Bits(), PrefixLen) and right-shifts to produce the offset.
func (p Pool) OffsetOf(addr netip.Addr) int {
	if !p.Prefix.Contains(addr) {
		return -1
	}

	base := p.Prefix.Addr().As16()
	target := addr.As16()
	addrBits := p.Prefix.Addr().BitLen()

	if p.PrefixLen == addrBits {
		// Fast path: individual address allocation.
		offset := 0
		for i := 0; i < 16; i++ {
			offset = (offset << 8) | (int(target[i]) - int(base[i]))
		}

		return offset
	}

	// Shifted allocation: compute (target - base) >> (addrBits - PrefixLen).
	baseInt := new(big.Int).SetBytes(base[:])
	targetInt := new(big.Int).SetBytes(target[:])
	diff := new(big.Int).Sub(targetInt, baseInt)
	diff.Rsh(diff, uint(addrBits-p.PrefixLen))

	return int(diff.Int64())
}

// addIntToIP adds a positive integer offset to a 16-byte IP address in-place.
func addIntToIP(ip []byte, offset int) {
	carry := offset
	for i := len(ip) - 1; i >= 0 && carry > 0; i-- {
		sum := int(ip[i]) + carry
		ip[i] = byte(sum & 0xFF)
		carry = sum >> 8
	}
}
