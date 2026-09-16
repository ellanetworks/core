// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build linux

// SPDX-License-Identifier: BUSL-1.1

package kernel

import (
	"fmt"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func routeTableForLink(link netlink.Link) (int, error) {
	if link == nil {
		return unix.RT_TABLE_MAIN, fmt.Errorf("nil link")
	}

	if link.Type() == "vrf" {
		if vrf, ok := link.(*netlink.Vrf); ok {
			if vrf.Table != 0 {
				return int(vrf.Table), nil
			}
		}

		return unix.RT_TABLE_MAIN, nil
	}

	masterIndex := link.Attrs().MasterIndex
	if masterIndex == 0 {
		return unix.RT_TABLE_MAIN, nil
	}

	master, err := kernelLinkByIndex(masterIndex)
	if err != nil {
		return unix.RT_TABLE_MAIN, fmt.Errorf("lookup master of interface %s: %w", link.Attrs().Name, err)
	}

	if master.Type() != "vrf" {
		return unix.RT_TABLE_MAIN, nil
	}

	if vrf, ok := master.(*netlink.Vrf); ok {
		if vrf.Table != 0 {
			return int(vrf.Table), nil
		}
	}

	return unix.RT_TABLE_MAIN, nil
}
