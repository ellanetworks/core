// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"net/netip"
	"sort"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const unusableAddrFlags = unix.IFA_F_TENTATIVE |
	unix.IFA_F_DADFAILED |
	unix.IFA_F_SECONDARY

func addrIsUsable(a netlink.Addr) (netip.Addr, bool) {
	if a.Flags&unusableAddrFlags != 0 {
		return netip.Addr{}, false
	}

	addr, ok := netip.AddrFromSlice(a.IP)
	if !ok {
		return netip.Addr{}, false
	}

	addr = addr.Unmap()

	if !addr.IsValid() || addr.IsUnspecified() ||
		addr.IsMulticast() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
		return netip.Addr{}, false
	}

	return addr, true
}

func addrRanksBefore(a, b netlink.Addr) bool {
	if a.Scope != b.Scope {
		return a.Scope < b.Scope
	}

	aDeprecated := a.Flags&unix.IFA_F_DEPRECATED != 0
	bDeprecated := b.Flags&unix.IFA_F_DEPRECATED != 0

	if aDeprecated != bDeprecated {
		return bDeprecated
	}

	aPermanent := a.Flags&unix.IFA_F_PERMANENT != 0
	bPermanent := b.Flags&unix.IFA_F_PERMANENT != 0

	if aPermanent != bPermanent {
		return aPermanent
	}

	if a.ValidLft != b.ValidLft {
		return a.ValidLft > b.ValidLft
	}

	return a.IP.String() < b.IP.String()
}

var UsableInterfaceAddrsFunc = func(name string) ([]netip.Addr, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}

	all, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return nil, err
	}

	usable := make([]netlink.Addr, 0, len(all))

	for _, a := range all {
		if _, ok := addrIsUsable(a); ok {
			usable = append(usable, a)
		}
	}

	sort.SliceStable(usable, func(i, j int) bool {
		return addrRanksBefore(usable[i], usable[j])
	})

	out := make([]netip.Addr, 0, len(usable))

	for _, a := range usable {
		addr, _ := addrIsUsable(a)
		out = append(out, addr)
	}

	return out, nil
}

func UsableInterfaceAddrs(name string) ([]netip.Addr, error) {
	return UsableInterfaceAddrsFunc(name)
}
