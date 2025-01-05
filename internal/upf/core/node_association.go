// Copyright 2024 Ella Networks

package core

import (
	"sync"
)

type NodeAssociation struct {
	ID               string
	Addr             string
	NextSessionID    uint64
	NextSequenceID   uint32
	Sessions         map[uint64]*Session
	FailedHeartbeats uint32
	sync.Mutex
}

func NewNodeAssociation(remoteNodeID string, addr string) *NodeAssociation {
	return &NodeAssociation{
		ID:             remoteNodeID,
		Addr:           addr,
		NextSessionID:  1,
		NextSequenceID: 1,
		Sessions:       make(map[uint64]*Session),
	}
}

func (association *NodeAssociation) NewLocalSEID() uint64 {
	association.NextSessionID += 1
	return association.NextSessionID
}

func (association *NodeAssociation) NewSequenceID() uint32 {
	association.NextSequenceID += 1
	return association.NextSequenceID
}

func (association *NodeAssociation) ResetFailedHeartbeats() {
	association.Lock()
	association.FailedHeartbeats = 0
	association.Unlock()
}
