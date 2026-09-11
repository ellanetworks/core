// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"fmt"
	"net/netip"
	"strings"
)

func ParseN3ExternalAddress(s string) (v4, v6 netip.Addr, err error) {
	fields := strings.SplitSeq(s, ",")

	for field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			return netip.Addr{}, netip.Addr{}, fmt.Errorf("external address %q: empty address", s)
		}

		addr, parseErr := netip.ParseAddr(field)
		if parseErr != nil {
			return netip.Addr{}, netip.Addr{}, fmt.Errorf("external address %q: %w", s, parseErr)
		}

		addr = addr.Unmap()

		if addr.Is4() {
			if v4.IsValid() {
				return netip.Addr{}, netip.Addr{}, fmt.Errorf("external address %q: more than one IPv4 address", s)
			}

			v4 = addr

			continue
		}

		if v6.IsValid() {
			return netip.Addr{}, netip.Addr{}, fmt.Errorf("external address %q: more than one IPv6 address", s)
		}

		v6 = addr
	}

	if !v4.IsValid() && !v6.IsValid() {
		return netip.Addr{}, netip.Addr{}, fmt.Errorf("external address %q: no address", s)
	}

	return v4, v6, nil
}
