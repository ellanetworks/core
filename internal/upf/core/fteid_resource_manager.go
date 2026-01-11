package core

import (
	"errors"
	"sync"
)

type FteIDResourceManager struct {
	free []uint32
	busy map[uint64]uint32 // seID -> teid
	sync.RWMutex
}

func NewFteIDResourceManager(teidRange uint32) (*FteIDResourceManager, error) {
	if teidRange == 0 {
		return nil, errors.New("TEID range should be greater than 0")
	}

	free := make([]uint32, 0, teidRange)

	for teid := uint32(1); teid <= teidRange; teid++ {
		free = append(free, teid)
	}

	return &FteIDResourceManager{
		free: free,
		busy: make(map[uint64]uint32),
	}, nil
}

func (m *FteIDResourceManager) AllocateTEID(seID uint64) (uint32, error) {
	m.Lock()
	defer m.Unlock()

	if _, exists := m.busy[seID]; exists {
		return 0, errors.New("TEID already allocated for seID")
	}

	if len(m.free) == 0 {
		return 0, errors.New("no free TEID available")
	}

	teid := m.free[0]
	m.free = m.free[1:]

	m.busy[seID] = teid

	return teid, nil
}

func (m *FteIDResourceManager) ReleaseTEID(seID uint64) {
	m.Lock()
	defer m.Unlock()

	if teid, ok := m.busy[seID]; ok {
		delete(m.busy, seID)
		m.free = append(m.free, teid)
	}
}
