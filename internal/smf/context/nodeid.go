// Copyright 2024 Ella Networks
// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Canonical Ltd.

package context

import (
	"net"
)

const NodeIDTypeIpv4Address uint8 = iota

type NodeID struct {
	NodeIDValue []byte
	NodeIDType  uint8 // 0x00001111
}

func NewNodeID(nodeID string) *NodeID {
	ip := net.ParseIP(nodeID)
	return &NodeID{
		NodeIDType:  NodeIDTypeIpv4Address,
		NodeIDValue: ip.To4(),
	}
}

func (n *NodeID) ResolveNodeIDToIP() net.IP {
	return n.NodeIDValue
}

func (n *NodeID) String() string {
	return net.IP(n.NodeIDValue).String()
}
