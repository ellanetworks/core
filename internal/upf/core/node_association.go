// Copyright 2024 Ella Networks

package core

import (
	"sync"
)

type NodeAssociation struct {
	ID            string
	NextSessionID uint64
	Sessions      map[uint64]*Session
	sync.Mutex
}

func NewNodeAssociation(remoteNodeID string, addr string) *NodeAssociation {
	return &NodeAssociation{
		ID:            remoteNodeID,
		NextSessionID: 1,
		Sessions:      make(map[uint64]*Session),
	}
}

func (association *NodeAssociation) NewLocalSEID() uint64 {
	association.NextSessionID += 1
	return association.NextSessionID
}
