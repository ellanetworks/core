// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Canonical Ltd.

package context

import "net"

const (
	OuterHeaderCreationGtpUUdpIpv4 uint16 = 256
	OuterHeaderRemovalGtpUUdpIpv4  uint8  = 0
)

type OuterHeaderRemoval struct {
	OuterHeaderRemovalDescription uint8
}

type OuterHeaderCreation struct {
	Ipv4Address                    net.IP
	Ipv6Address                    net.IP
	Teid                           uint32
	PortNumber                     uint16
	OuterHeaderCreationDescription uint16
}
