// Copyright 2024 Ella Networks
package core

import (
	"errors"
	"sync"
)

type ResourceManager struct {
	FTEIDM *FTEIDM
}

type FTEIDM struct {
	freeTEIDs []uint32
	busyTEIDs map[uint64]map[uint32]uint32 // map[seID]map[pdrID]teid
	sync.RWMutex
}

func NewResourceManager(teidRange uint32) (*ResourceManager, error) {
	var fteidm FTEIDM

	if teidRange != 0 {
		freeTEIDs := make([]uint32, 0, 10000)
		busyTEIDs := make(map[uint64]map[uint32]uint32)

		var teid uint32

		for teid = 1; teid <= teidRange; teid++ {
			freeTEIDs = append(freeTEIDs, teid)
		}

		fteidm = FTEIDM{
			freeTEIDs: freeTEIDs,
			busyTEIDs: busyTEIDs,
		}
	}

	return &ResourceManager{
		FTEIDM: &fteidm,
	}, nil
}

func (fteidm *FTEIDM) AllocateTEID(seID uint64, pdrID uint32) (uint32, error) {
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

func (fteidm *FTEIDM) ReleaseTEID(seID uint64) {
	fteidm.Lock()
	defer fteidm.Unlock()

	if teid, ok := fteidm.busyTEIDs[seID]; ok {
		for _, t := range teid {
			fteidm.freeTEIDs = append(fteidm.freeTEIDs, t)
		}
		delete(fteidm.busyTEIDs, seID)
	}
}
