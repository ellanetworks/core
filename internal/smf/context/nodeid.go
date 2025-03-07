// Copyright 2024 Ella Networks
// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Canonical Ltd.

package context

import (
	"net"
)

type NodeID struct {
	Value []byte
}

func NewNodeID(nodeID string) *NodeID {
	ip := net.ParseIP(nodeID)
	return &NodeID{
		Value: ip.To4(),
	}
}

func (n *NodeID) ResolveNodeIdToIp() net.IP {
	return n.Value
}

func (n *NodeID) String() string {
	return net.IP(n.Value).String()
}
