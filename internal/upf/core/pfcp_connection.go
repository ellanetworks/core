// Copyright 2024 Ella Networks
package core

import (
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

var connection *PfcpConnection

type PfcpConnection struct {
	SmfNodeAssociation   *NodeAssociation
	SmfAddress           string
	nodeID               string
	nodeAddrV4           net.IP
	n3Address            net.IP
	bpfObjects           *ebpf.BpfObjects
	FteIDResourceManager *FteIDResourceManager
}

func CreatePfcpConnection(addr string, nodeID string, n3Ip string, smfAddress string, bpfObjects *ebpf.BpfObjects, resourceManager *FteIDResourceManager) (*PfcpConnection, error) {
	addrV4 := net.ParseIP(addr)
	if addrV4 == nil {
		return nil, fmt.Errorf("failed to parse IP address ID: %s", addr)
	}
	n3Addr := net.ParseIP(n3Ip)
	if n3Addr == nil {
		return nil, fmt.Errorf("failed to parse N3 IP address ID: %s", n3Ip)
	}

	connection = &PfcpConnection{
		nodeID:               nodeID,
		nodeAddrV4:           addrV4,
		n3Address:            n3Addr,
		bpfObjects:           bpfObjects,
		FteIDResourceManager: resourceManager,
		SmfAddress:           smfAddress,
	}

	return connection, nil
}

func GetConnection() *PfcpConnection {
	return connection
}

func (connection *PfcpConnection) ReleaseResources(seID uint64) {
	if connection.FteIDResourceManager == nil {
		return
	}

	if connection.FteIDResourceManager != nil {
		connection.FteIDResourceManager.ReleaseTEID(seID)
	}
}
