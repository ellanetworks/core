// Copyright 2024 Ella Networks
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
	IPv4Address                    net.IP
	IPv6Address                    net.IP
	TeID                           uint32
	PortNumber                     uint16
	OuterHeaderCreationDescription uint16
}
