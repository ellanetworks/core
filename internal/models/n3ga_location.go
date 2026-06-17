// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package models

type N3gaLocation struct {
	N3gppTai   *Tai
	UeIpv4Addr string
	UeIpv6Addr string
	PortNumber int32
}
