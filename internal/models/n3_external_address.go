// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"fmt"
	"net/netip"
	"strings"
)

// ParseN3ExternalAddress parses the operator-configured N3 / S1-U external
// address into the endpoints to advertise in the Transport Layer Address. It
// accepts a single IPv4 address, a single IPv6 address, or one of each
// separated by a comma. A family the operator did not name comes back as the
// zero Addr, so the advertisement carries exactly the families that were
// configured and nothing else.
func ParseN3ExternalAddress(s string) (v4, v6 netip.Addr, err error) {
	fields := strings.Split(s, ",")

	for _, field := range fields {
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
