// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf_test

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/upf"
)

func TestRegisterIPv6SessionWithoutAResponderIsANoOp(t *testing.T) {
	var u upf.UPF

	u.RegisterIPv6Session(0xBEEF, &upf.IPv6SessionContext{
		Prefix: netip.MustParsePrefix("2001:db8:1::/64"),
		MTU:    1400,
	})
	u.UnregisterIPv6Session(0xBEEF)
}
