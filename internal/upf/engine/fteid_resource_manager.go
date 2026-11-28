// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
)

type FteIDResourceManager struct {
	capacity uint32
	inUse    map[uint32]struct{}
	busy     map[uint64]map[uint32]struct{} // seID -> allocated TEIDs
	mu       sync.Mutex
}

func NewFteIDResourceManager(teidRange uint32) (*FteIDResourceManager, error) {
	if teidRange == 0 {
		return nil, fmt.Errorf("TEID range should be greater than 0")
	}

	return &FteIDResourceManager{
		capacity: teidRange,
		inUse:    make(map[uint32]struct{}),
		busy:     make(map[uint64]map[uint32]struct{}),
	}, nil
}

func (m *FteIDResourceManager) AllocateTEID(seID uint64) (uint32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if uint64(len(m.inUse)) >= uint64(m.capacity) {
		return 0, fmt.Errorf("no free TEID available")
	}

	teid, err := m.randomFreeTEID()
	if err != nil {
		return 0, err
	}

	m.inUse[teid] = struct{}{}

	teids := m.busy[seID]
	if teids == nil {
		teids = make(map[uint32]struct{})
		m.busy[seID] = teids
	}

	teids[teid] = struct{}{}

	return teid, nil
}

func (m *FteIDResourceManager) randomFreeTEID() (uint32, error) {
	var buf [4]byte

	for {
		if _, err := rand.Read(buf[:]); err != nil {
			return 0, fmt.Errorf("couldn't generate TEID: %w", err)
		}

		teid := binary.BigEndian.Uint32(buf[:])
		if teid == 0 {
			continue
		}

		if _, taken := m.inUse[teid]; taken {
			continue
		}

		return teid, nil
	}
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

	delete(m.inUse, teid)
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
		delete(m.inUse, teid)
	}

	delete(m.busy, seID)
}
