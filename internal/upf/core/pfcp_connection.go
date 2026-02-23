// Copyright 2024 Ella Networks
package core

import (
	"fmt"
	"maps"
	"net"
	"sync"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

var connection *PfcpConnection

type PfcpConnection struct {
	mu sync.Mutex

	sessions             map[uint64]*Session
	nodeID               string
	nodeAddrV4           net.IP
	n3Address            net.IP
	advertisedN3Address  net.IP
	BpfObjects           *ebpf.BpfObjects
	FteIDResourceManager *FteIDResourceManager
}

func (pc *PfcpConnection) ListSessions() map[uint64]*Session {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	copy := make(map[uint64]*Session, len(pc.sessions))
	maps.Copy(copy, pc.sessions)
	return copy
}

func (pc *PfcpConnection) GetSession(seid uint64) *Session {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	session, ok := pc.sessions[seid]
	if !ok {
		return nil
	}

	return session
}

func (pc *PfcpConnection) DeleteSession(seid uint64) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	delete(pc.sessions, seid)
}

func (pc *PfcpConnection) AddSession(seid uint64, session *Session) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.sessions[seid] = session
}

func (pc *PfcpConnection) SetBPFObjects(bpfObjects *ebpf.BpfObjects) {
	pc.BpfObjects = bpfObjects
}

func (pc *PfcpConnection) GetAdvertisedN3Address() net.IP {
	return pc.advertisedN3Address
}

func (pc *PfcpConnection) SetAdvertisedN3Address(newN3Addr net.IP) {
	pc.advertisedN3Address = newN3Addr
}

func CreatePfcpConnection(addr string, nodeID string, n3Ip string, advertisedN3Ip string, bpfObjects *ebpf.BpfObjects, resourceManager *FteIDResourceManager) (*PfcpConnection, error) {
	addrV4 := net.ParseIP(addr)
	if addrV4 == nil {
		return nil, fmt.Errorf("failed to parse IP address ID: %s", addr)
	}

	n3Addr := net.ParseIP(n3Ip)
	if n3Addr == nil {
		return nil, fmt.Errorf("failed to parse N3 IP address ID: %s", n3Ip)
	}

	advertisedN3Addr := net.ParseIP(advertisedN3Ip)
	if advertisedN3Addr == nil {
		return nil, fmt.Errorf("failed to parse advertised N3 IP address ID: %s", advertisedN3Ip)
	}

	connection = &PfcpConnection{
		sessions:             make(map[uint64]*Session),
		nodeID:               nodeID,
		nodeAddrV4:           addrV4,
		n3Address:            n3Addr,
		advertisedN3Address:  advertisedN3Addr,
		BpfObjects:           bpfObjects,
		FteIDResourceManager: resourceManager,
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
