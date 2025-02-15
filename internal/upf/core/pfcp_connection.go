// Copyright 2024 Ella Networks
package core

import (
	"fmt"
	"net"
	"time"

	"github.com/ellanetworks/core/internal/upf/core/service"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

var connection *PfcpConnection

type PfcpConnection struct {
	NodeAssociations  map[string]*NodeAssociation
	nodeId            string
	nodeAddrV4        net.IP
	n3Address         net.IP
	bpfObjects        ebpf.BpfObjects
	RecoveryTimestamp time.Time
	ResourceManager   *service.ResourceManager
}

func CreatePfcpConnection(addr string, nodeId string, n3Ip string, bpfObjects ebpf.BpfObjects, resourceManager *service.ResourceManager) (*PfcpConnection, error) {
	addrV4 := net.ParseIP(addr)
	if addrV4 == nil {
		return nil, fmt.Errorf("failed to parse IP address ID: %s", addr)
	}
	n3Addr := net.ParseIP(n3Ip)
	if n3Addr == nil {
		return nil, fmt.Errorf("failed to parse N3 IP address ID: %s", n3Ip)
	}

	connection = &PfcpConnection{
		NodeAssociations:  map[string]*NodeAssociation{},
		nodeId:            nodeId,
		nodeAddrV4:        addrV4,
		n3Address:         n3Addr,
		bpfObjects:        bpfObjects,
		RecoveryTimestamp: time.Now(),
		ResourceManager:   resourceManager,
	}

	return connection, nil
}

func GetConnection() *PfcpConnection {
	return connection
}

func (connection *PfcpConnection) ReleaseResources(seID uint64) {
	if connection.ResourceManager == nil {
		return
	}

	if connection.ResourceManager.IPAM != nil {
		connection.ResourceManager.IPAM.ReleaseIP(seID)
	}

	if connection.ResourceManager.FTEIDM != nil {
		connection.ResourceManager.FTEIDM.ReleaseTEID(seID)
	}
}
