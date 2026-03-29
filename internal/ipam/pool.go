// Copyright 2026 Ella Networks

package ipam

import (
	"fmt"
	"net/netip"
)

// Pool describes an IP address pool backed by a data network's CIDR.
// It provides methods to compute usable address ranges and convert between
// offsets and addresses. The offset is relative to the network base address
// (e.g. for 10.0.0.0/24, offset 1 = 10.0.0.1).
type Pool struct {
	// ID is the data_networks.id primary key, used as pool_id in ip_leases.
	ID int

	// Prefix is the parsed CIDR (e.g. "10.45.0.0/22" → 10.45.0.0/22).
	Prefix netip.Prefix
}

// NewPool creates a Pool from a data network ID and CIDR string.
func NewPool(id int, cidr string) (Pool, error) {
	prefix, err := netip.ParsePrefix(cidr)
	if err != nil {
		return Pool{}, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}

	// Normalize to the masked form (e.g. 10.45.0.1/22 → 10.45.0.0/22).
	prefix = netip.PrefixFrom(prefix.Masked().Addr(), prefix.Bits())

	return Pool{ID: id, Prefix: prefix}, nil
}

// FirstUsable returns the offset of the first usable host address.
// For IPv4 this is 1 (skipping the network address).
// For IPv6 this is 0 (no network/broadcast concept for individual addresses).
func (p Pool) FirstUsable() int {
	if p.Prefix.Addr().Is4() {
		return 1
	}

	return 0
}

// Size returns the total number of addresses in the pool, excluding
// network and broadcast for IPv4.
// For a /24: 256 total − network − broadcast = 254 usable.
// For a /22: 1024 − 2 = 1022 usable.
func (p Pool) Size() int {
	bits := p.Prefix.Addr().BitLen() // 32 for IPv4, 128 for IPv6
	hostBits := bits - p.Prefix.Bits()

	if hostBits >= 31 {
		// Very large pool — cap at max int to avoid overflow.
		// RandomAllocator (future) handles these; SequentialAllocator
		// is not used for pools this large.
		return int(^uint(0) >> 1)
	}

	total := 1 << hostBits

	if p.Prefix.Addr().Is4() {
		// Subtract network and broadcast addresses.
		return total - 2
	}

	return total
}

// AddressAtOffset returns the address at the given offset from the base
// network address. Offset 0 = base address, offset 1 = base+1, etc.
// The caller is responsible for ensuring offset is within [FirstUsable, FirstUsable+Size).
func (p Pool) AddressAtOffset(offset int) netip.Addr {
	base := p.Prefix.Addr()

	// Convert the base address to a 16-byte representation and add the offset.
	b := base.As16()
	addIntToIP(b[:], offset)

	if base.Is4() {
		return netip.AddrFrom4([4]byte(b[12:16]))
	}

	return netip.AddrFrom16(b)
}

// OffsetOf returns the offset of addr relative to the pool's base address.
// Returns -1 if addr is not within the pool's prefix.
func (p Pool) OffsetOf(addr netip.Addr) int {
	if !p.Prefix.Contains(addr) {
		return -1
	}

	base := p.Prefix.Addr().As16()
	target := addr.As16()

	offset := 0
	for i := 0; i < 16; i++ {
		offset = (offset << 8) | (int(target[i]) - int(base[i]))
	}

	return offset
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
