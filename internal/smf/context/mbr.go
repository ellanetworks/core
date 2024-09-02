// Copyright 2024 Canonical Ltd.
//
// SPDX-License-Identifier: Apache-2.0

package context

type MBR struct {
	ULMBR uint64 // 40-bit data
	DLMBR uint64 // 40-bit data
}
