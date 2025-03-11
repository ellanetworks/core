// Copyright 2024 Ella Networks
// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Canonical Ltd.

package context

import (
	"net"
)

type NodeID struct {
	NodeIDValue []byte
}

func NewNodeID(nodeID string) *NodeID {
	ip := net.ParseIP(nodeID)
	return &NodeID{
		NodeIDValue: ip.To4(),
	}
}

func (n *NodeID) ResolveNodeIDToIP() net.IP {
	return n.NodeIDValue
}

func (n *NodeID) String() string {
	return net.IP(n.NodeIDValue).String()
}
