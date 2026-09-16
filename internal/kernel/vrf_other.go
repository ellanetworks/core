// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build !linux

// SPDX-License-Identifier: BUSL-1.1

package kernel

import (
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func routeTableForLink(link netlink.Link) (int, error) {
	return unix.RT_TABLE_MAIN, nil
}
