// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Canonical Ltd.

package context

import (
	"net"
)

const NodeIdTypeIpv4Address uint8 = iota

type NodeID struct {
	NodeIdValue []byte
	NodeIdType  uint8 // 0x00001111
}

func NewNodeID(nodeID string) *NodeID {
	ip := net.ParseIP(nodeID)
	return &NodeID{
		NodeIdType:  NodeIdTypeIpv4Address,
		NodeIdValue: ip.To4(),
	}
}

func (n *NodeID) ResolveNodeIdToIp() net.IP {
	return n.NodeIdValue
}

func (n *NodeID) String() string {
	return net.IP(n.NodeIdValue).String()
}
