// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"fmt"
	"sync"
)

type FteIDResourceManager struct {
	free []uint32
	busy map[uint64]map[uint32]struct{} // seID -> allocated TEIDs
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
		busy: make(map[uint64]map[uint32]struct{}),
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

	teids := m.busy[seID]
	if teids == nil {
		teids = make(map[uint32]struct{})
		m.busy[seID] = teids
	}

	teids[teid] = struct{}{}

	return teid, nil
}

// ReleaseTEID returns one TEID to the pool. A TEID not currently allocated to
// the session is ignored, so a double release cannot hand the same TEID to two
// sessions.
func (m *FteIDResourceManager) ReleaseTEID(seID uint64, teid uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()

	teids, ok := m.busy[seID]
	if !ok {
		return
	}

	if _, ok := teids[teid]; !ok {
		return
	}

	delete(teids, teid)

	if len(teids) == 0 {
		delete(m.busy, seID)
	}

	m.free = append(m.free, teid)
}

// ReleaseAllTEIDs returns every TEID a session still holds to the pool. It is a
// teardown backstop so a missed per-PDR release cannot exhaust the pool.
func (m *FteIDResourceManager) ReleaseAllTEIDs(seID uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	teids, ok := m.busy[seID]
	if !ok {
		return
	}

	for teid := range teids {
		m.free = append(m.free, teid)
	}

	delete(m.busy, seID)
}
