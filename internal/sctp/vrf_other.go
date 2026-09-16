// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build !linux

// SPDX-License-Identifier: BUSL-1.1

package sctp

import (
	"errors"
	"syscall"
)

func vrfMasterDevice(ifaceName string) (string, error) {
	return "", nil
}

func vrfDeviceForAddress(ipStr string) (string, error) {
	return "", nil
}

func vrfBindDevice(interfaceName, address string) (string, error) {
	return "", nil
}

func bindToDeviceControl(device string) func(network, address string, c syscall.RawConn) error {
	return func(network, address string, c syscall.RawConn) error {
		return errors.New("sctp: SO_BINDTODEVICE is only supported on Linux")
	}
}
