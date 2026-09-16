// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build !linux

// SPDX-License-Identifier: BUSL-1.1

package listener

import (
	"syscall"
)

func vrfDeviceForBindAddress(bindAddr string) string {
	return ""
}

func vrfDeviceForDestination(addr string) string {
	return ""
}

func bindToDeviceControl(device string) func(network, address string, c syscall.RawConn) error {
	return nil
}
