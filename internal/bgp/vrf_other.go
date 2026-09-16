// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build !linux

// SPDX-License-Identifier: BUSL-1.1

package bgp

func vrfMasterDevice(ifaceName string) (string, error) {
	return "", nil
}

func vrfDeviceForAddress(ipStr string) (string, error) {
	return "", nil
}
