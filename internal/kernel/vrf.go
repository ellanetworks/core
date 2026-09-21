// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package kernel

import (
	"fmt"

	"github.com/ellanetworks/core/internal/netutil"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func routeTableForLink(link netlink.Link) (int, error) {
	if link == nil {
		return unix.RT_TABLE_MAIN, fmt.Errorf("nil link")
	}

	master, err := netutil.VRFMasterOf(link)
	if err != nil {
		return unix.RT_TABLE_MAIN, err
	}

	if master == nil {
		return unix.RT_TABLE_MAIN, nil
	}

	if vrf, ok := master.(*netlink.Vrf); ok && vrf.Table != 0 {
		return int(vrf.Table), nil
	}

	return unix.RT_TABLE_MAIN, nil
}
