// Copyright 2026 Ella Networks

package core

import (
	"fmt"
	"sync"
)

type FteIDResourceManager struct {
	free []uint32
	busy map[uint64]uint32 // seID -> teid
	mu   sync.Mutex
}

func NewFteIDResourceManager(teidRange uint32) (*FteIDResourceManager, error) {
	if teidRange == 0 {
		return nil, fmt.Errorf("TEID range should be greater than 0")
	}

	free := make([]uint32, 0, teidRange)

	for i := range teidRange {
		free = append(free, i+1)
	}

	return &FteIDResourceManager{
		free: free,
		busy: make(map[uint64]uint32),
	}, nil
}

func (m *FteIDResourceManager) AllocateTEID(seID uint64) (uint32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.free) == 0 {
		return 0, fmt.Errorf("no free TEID available")
	}

	teid := m.free[0]
	m.free = m.free[1:]

	m.busy[seID] = teid

	return teid, nil
}

func (m *FteIDResourceManager) ReleaseTEID(seID uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	teid, ok := m.busy[seID]
	if !ok {
		return
	}

	delete(m.busy, seID)
	m.free = append(m.free, teid)
}
