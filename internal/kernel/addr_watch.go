// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package kernel

import (
	"context"
	"fmt"
	"time"

	"github.com/vishvananda/netlink"
)

func WatchInterfaceAddrs(ctx context.Context, ifaceName string, debounce time.Duration, onChange func()) error {
	link, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("cannot resolve interface %s: %w", ifaceName, err)
	}

	index := link.Attrs().Index

	updates := make(chan netlink.AddrUpdate, 64)

	err = netlink.AddrSubscribeWithOptions(updates, ctx.Done(), netlink.AddrSubscribeOptions{
		ErrorCallback: func(error) {},
	})
	if err != nil {
		return fmt.Errorf("cannot subscribe to address updates: %w", err)
	}

	go watchAddrUpdates(ctx, updates, index, debounce, onChange)

	return nil
}

func watchAddrUpdates(ctx context.Context, updates <-chan netlink.AddrUpdate, index int, debounce time.Duration, onChange func()) {
	for {
		select {
		case <-ctx.Done():
			return
		case upd, ok := <-updates:
			if !ok {
				return
			}

			if upd.LinkIndex != index {
				continue
			}

			if !coalesce(ctx, updates, debounce) {
				return
			}

			onChange()
		}
	}
}

func coalesce(ctx context.Context, updates <-chan netlink.AddrUpdate, debounce time.Duration) bool {
	timer := time.NewTimer(debounce)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case _, ok := <-updates:
			if !ok {
				return false
			}
		case <-timer.C:
			return true
		}
	}
}
