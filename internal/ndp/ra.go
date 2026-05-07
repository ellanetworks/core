// Copyright 2026 Ella Networks

package ndp

import (
	"encoding/binary"
	"errors"
	"net/netip"
)

// RA header layout (RFC 4861 §4.2):
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     Type      |     Code      |          Checksum             |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| Cur Hop Limit |M|O| Reserved  |       Router Lifetime         |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                         Reachable Time                        |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                          Retrans Timer                        |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|   Options ...
//	+-+-+-+-+-+-+-+-+-+-+-+-
//
// Fixed header: 16 bytes (type + code + checksum + curHopLimit + flags + routerLifetime + reachableTime + retransTimer).

const (
	raHeaderLen = 16

	// Prefix Information Option is 32 bytes (type + length + prefix_len + flags + valid + preferred + reserved + prefix).
	prefixInfoOptionLen = 32

	// MTU Option is 8 bytes (type + length + reserved + MTU).
	mtuOptionLen = 8
)

// RAParams contains the parameters needed to build a Router Advertisement.
type RAParams struct {
	// SrcIP is the link-local source IPv6 address for the RA. Required.
	SrcIP netip.Addr

	// DstIP is the destination IPv6 address for the RA. Typically the
	// RS source address (unicast reply) or ff02::1 (all-nodes multicast).
	DstIP netip.Addr

	// CurHopLimit is the default hop limit to advertise to hosts. 0 means
	// unspecified by this router.
	CurHopLimit uint8

	// Managed indicates the M (Managed address configuration) flag.
	Managed bool

	// Other indicates the O (Other configuration) flag.
	Other bool

	// RouterLifetime is the lifetime in seconds associated with the default
	// router. 0 means this router is not a default router.
	RouterLifetime uint16

	// ReachableTime is the time in milliseconds that a neighbor is
	// considered reachable after a reachability confirmation. 0 = unspecified.
	ReachableTime uint32

	// RetransTimer is the time in milliseconds between retransmitted
	// Neighbor Solicitation messages. 0 = unspecified.
	RetransTimer uint32

	// Prefix is the delegated /64 prefix to advertise in the Prefix
	// Information Option.
	Prefix netip.Prefix

	// OnLink indicates the L (on-link) flag in the Prefix Information Option.
	OnLink bool

	// Autonomous indicates the A (autonomous address configuration) flag
	// in the Prefix Information Option (SLAAC).
	Autonomous bool

	// ValidLifetime is the length of time in seconds that the prefix is
	// valid for on-link determination. 0xFFFFFFFF = infinity.
	ValidLifetime uint32

	// PreferredLifetime is the length of time in seconds that addresses
	// generated from the prefix via SLAAC are preferred. Must be <= ValidLifetime.
	PreferredLifetime uint32

	// MTU is the link MTU to advertise in the MTU option. 0 omits the option.
	MTU uint32
}

var (
	ErrMissingPrefix = errors.New("ndp: prefix is required")
	ErrNotIPv6       = errors.New("ndp: addresses must be IPv6")
)

// BuildRA constructs a Router Advertisement ICMPv6 message body. The returned
// bytes start at the ICMPv6 Type field. The checksum field is set to zero;
// callers must compute it using the IPv6 pseudo-header (see ICMPv6Checksum).
//
// The message includes:
//   - RA fixed header (16 bytes)
//   - Prefix Information Option (32 bytes) with the delegated prefix
//   - MTU Option (8 bytes) if params.MTU > 0
func BuildRA(params RAParams) ([]byte, error) {
	if !params.Prefix.IsValid() {
		return nil, ErrMissingPrefix
	}

	if !params.SrcIP.Is6() || !params.DstIP.Is6() {
		return nil, ErrNotIPv6
	}

	size := raHeaderLen + prefixInfoOptionLen
	if params.MTU > 0 {
		size += mtuOptionLen
	}

	buf := make([]byte, size)

	// -- RA fixed header --
	buf[0] = ICMPv6TypeRouterAdvertisement // Type
	buf[1] = 0                             // Code
	// buf[2:4] = checksum (left as zero for caller to fill)
	buf[4] = params.CurHopLimit

	var flags byte
	if params.Managed {
		flags |= 0x80 // M flag
	}

	if params.Other {
		flags |= 0x40 // O flag
	}

	buf[5] = flags

	binary.BigEndian.PutUint16(buf[6:8], params.RouterLifetime)
	binary.BigEndian.PutUint32(buf[8:12], params.ReachableTime)
	binary.BigEndian.PutUint32(buf[12:16], params.RetransTimer)

	// -- Prefix Information Option (RFC 4861 §4.6.2) --
	off := raHeaderLen
	encodePrefixInfoOption(buf[off:off+prefixInfoOptionLen], params)
	off += prefixInfoOptionLen

	// -- MTU Option (RFC 4861 §4.6.4) --
	if params.MTU > 0 {
		encodeMTUOption(buf[off:off+mtuOptionLen], params.MTU)
	}

	return buf, nil
}

// encodePrefixInfoOption writes a 32-byte Prefix Information Option into dst.
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     Type(3)   |   Length(4)   | Prefix Length |L|A|  Rsvd1    |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                         Valid Lifetime                         |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                       Preferred Lifetime                       |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                           Reserved2                            |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                                                               |
//	+                            Prefix (16 bytes)                  +
//	|                                                               |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
func encodePrefixInfoOption(dst []byte, p RAParams) {
	dst[0] = NDPOptionPrefixInformation // Type
	dst[1] = 4                          // Length in 8-octet units (32 bytes / 8 = 4)
	dst[2] = uint8(p.Prefix.Bits())     // Prefix Length

	var flags byte
	if p.OnLink {
		flags |= 0x80 // L flag
	}

	if p.Autonomous {
		flags |= 0x40 // A flag
	}

	dst[3] = flags

	binary.BigEndian.PutUint32(dst[4:8], p.ValidLifetime)
	binary.BigEndian.PutUint32(dst[8:12], p.PreferredLifetime)
	// dst[12:16] = Reserved2 (zero)

	// Prefix: 16 bytes at offset 16.
	prefixAddr := p.Prefix.Masked().Addr()
	addr16 := prefixAddr.As16()
	copy(dst[16:32], addr16[:])
}

// encodeMTUOption writes an 8-byte MTU Option into dst.
//
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     Type(5)   |   Length(1)   |           Reserved            |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|                              MTU                              |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
func encodeMTUOption(dst []byte, mtu uint32) {
	dst[0] = NDPOptionMTU // Type
	dst[1] = 1            // Length in 8-octet units (8 bytes / 8 = 1)
	// dst[2:4] = Reserved (zero)
	binary.BigEndian.PutUint32(dst[4:8], mtu)
}

// ICMPv6Checksum computes the ICMPv6 checksum over the IPv6 pseudo-header
// and the ICMPv6 message body per RFC 4443 §2.3 and RFC 2460 §8.1.
//
// The checksum field in the message (bytes 2-3) must be zero before calling.
//
// Pseudo-header:
//
//	Source Address      (16 bytes)
//	Destination Address (16 bytes)
//	Upper-Layer Length   (4 bytes, big-endian)
//	Zero + Next Header   (4 bytes: 3 zero bytes + 58)
func ICMPv6Checksum(src, dst netip.Addr, icmpv6Body []byte) uint16 {
	src16 := src.As16()
	dst16 := dst.As16()

	var sum uint32

	// Source address
	for i := 0; i < 16; i += 2 {
		sum += uint32(src16[i])<<8 | uint32(src16[i+1]) // #nosec: G602 -- src16 is fixed-size 16-byte array
	}
	// Destination address
	for i := 0; i < 16; i += 2 {
		sum += uint32(dst16[i])<<8 | uint32(dst16[i+1]) // #nosec: G602 -- dst16 is fixed-size 16-byte array
	}
	// Upper-layer packet length (32-bit, big-endian)
	bodyLen := uint32(len(icmpv6Body))
	sum += bodyLen >> 16
	sum += bodyLen & 0xFFFF
	// Next Header (ICMPv6 = 58)
	sum += uint32(ICMPv6ProtocolNumber)

	// ICMPv6 body
	for i := 0; i+1 < len(icmpv6Body); i += 2 {
		sum += uint32(icmpv6Body[i])<<8 | uint32(icmpv6Body[i+1])
	}
	// Odd byte
	if len(icmpv6Body)%2 != 0 {
		sum += uint32(icmpv6Body[len(icmpv6Body)-1]) << 8
	}

	// Fold 32-bit sum into 16 bits
	for sum > 0xFFFF {
		sum = (sum >> 16) + (sum & 0xFFFF)
	}

	return ^uint16(sum)
}

// SetICMPv6Checksum computes and writes the ICMPv6 checksum into the
// message body (bytes 2-3). The checksum field must be zero before calling.
func SetICMPv6Checksum(src, dst netip.Addr, icmpv6Body []byte) {
	csum := ICMPv6Checksum(src, dst, icmpv6Body)
	binary.BigEndian.PutUint16(icmpv6Body[2:4], csum)
}
