// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package tracing

import (
	"errors"
	"net"
	"net/netip"

	"github.com/ellanetworks/core/sctp"
)

var ErrNoTCEAddress = errors.New("tracing: association has no usable local address")

type AssociationAddrs interface {
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

var routeSource = func(dst netip.Addr) (netip.Addr, bool) {
	c, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(netip.AddrPortFrom(dst, 9)))
	if err != nil {
		return netip.Addr{}, false
	}

	defer func() { _ = c.Close() }()

	la, ok := c.LocalAddr().(*net.UDPAddr)
	if !ok {
		return netip.Addr{}, false
	}

	return la.AddrPort().Addr().Unmap(), true
}

func TCEAddress(conn any) (netip.Addr, error) {
	assoc, ok := conn.(AssociationAddrs)
	if !ok {
		return netip.Addr{}, ErrNoTCEAddress
	}

	local := sctpAddrs(assoc.LocalAddr())
	remote := sctpAddrs(assoc.RemoteAddr())

	var peer netip.Addr
	if len(remote) > 0 {
		peer = remote[0]
	}

	var candidates []netip.Addr

	for _, addr := range local {
		if usableTCE(addr, peer) {
			candidates = append(candidates, addr)
		}
	}

	if len(candidates) == 0 {
		return netip.Addr{}, ErrNoTCEAddress
	}

	if !peer.IsValid() {
		return candidates[0], nil
	}

	if src, ok := routeSource(peer); ok {
		for _, addr := range candidates {
			if addr == src {
				return addr, nil
			}
		}
	}

	for _, addr := range candidates {
		if addr.Is4() == peer.Is4() {
			return addr, nil
		}
	}

	return candidates[0], nil
}

func usableTCE(addr, peer netip.Addr) bool {
	if !addr.IsValid() || addr.IsUnspecified() || addr.IsMulticast() ||
		addr.IsLinkLocalUnicast() {
		return false
	}

	return !addr.IsLoopback() || (peer.IsValid() && peer.IsLoopback())
}

func sctpAddrs(a net.Addr) []netip.Addr {
	sa, ok := a.(*sctp.SCTPAddr)
	if !ok || sa == nil {
		return nil
	}

	out := make([]netip.Addr, 0, len(sa.IPAddrs))

	for _, ip := range sa.IPAddrs {
		if addr, ok := netip.AddrFromSlice(ip.IP); ok {
			out = append(out, addr.Unmap())
		}
	}

	return out
}
