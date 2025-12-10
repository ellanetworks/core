// Copyright 2024 Ella Networks
package core

import (
	"errors"
	"sync"
)

type FteIDResourceManager struct {
	freeTEIDs []uint32
	busyTEIDs map[uint64]map[uint32]uint32 // map[seID]map[pdrID]teid
	sync.Mutex
}

func NewFteIDResourceManager(teidRange uint32) (*FteIDResourceManager, error) {
	if teidRange == 0 {
		return nil, errors.New("TEID range should be greater than 0")
	}

	freeTEIDs := make([]uint32, 0, 10000)
	busyTEIDs := make(map[uint64]map[uint32]uint32)

	var teid uint32

	for teid = 1; teid <= teidRange; teid++ {
		freeTEIDs = append(freeTEIDs, teid)
	}

	fteidm := &FteIDResourceManager{
		freeTEIDs: freeTEIDs,
		busyTEIDs: busyTEIDs,
	}

	return fteidm, nil
}

func (fteidm *FteIDResourceManager) AllocateTEID(seID uint64, pdrID uint32) (uint32, error) {
	fteidm.Lock()
	defer fteidm.Unlock()

	if len(fteidm.freeTEIDs) > 0 {
		teid := fteidm.freeTEIDs[0]
		fteidm.freeTEIDs = fteidm.freeTEIDs[1:]
		if _, ok := fteidm.busyTEIDs[seID]; !ok {
			pdr := make(map[uint32]uint32)
			pdr[pdrID] = teid
			fteidm.busyTEIDs[seID] = pdr
		} else {
			fteidm.busyTEIDs[seID][pdrID] = teid
		}
		return teid, nil
	} else {
		return 0, errors.New("no free TEID available")
	}
}

func (fteidm *FteIDResourceManager) ReleaseTEID(seID uint64) {
	fteidm.Lock()
	defer fteidm.Unlock()

	if teid, ok := fteidm.busyTEIDs[seID]; ok {
		for _, t := range teid {
			fteidm.freeTEIDs = append(fteidm.freeTEIDs, t)
		}
		delete(fteidm.busyTEIDs, seID)
	}
}
