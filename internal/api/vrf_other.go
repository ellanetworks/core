// SPDX-FileCopyrightText: Ella Networks Inc.
//go:build !linux

// SPDX-License-Identifier: BUSL-1.1

package api

func vrfDeviceForAPIAddress(address string) string {
	return ""
}
