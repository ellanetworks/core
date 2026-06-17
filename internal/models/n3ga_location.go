// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package models

type N3gaLocation struct {
	N3gppTai   *Tai
	UeIpv4Addr string
	UeIpv6Addr string
	PortNumber int32
}
