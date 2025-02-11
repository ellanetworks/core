// Copyright 2024 Ella Networks
package service

import (
	"errors"
	"net"
	"sync"
)

type ResourceManager struct {
	IPAM   *IPAM
	FTEIDM *FTEIDM
}

type FTEIDM struct {
	freeTEIDs []uint32
	busyTEIDs map[uint64]map[uint32]uint32 // map[seID]map[pdrID]teid
	sync.RWMutex
}

type IPAM struct {
	freeIPs []net.IP
	busyIPs map[uint64]net.IP
	sync.RWMutex
}

func NewResourceManager(teidRange uint32) (*ResourceManager, error) {
	var ipam IPAM
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
		IPAM:   &ipam,
		FTEIDM: &fteidm,
	}, nil
}

func (ipam *IPAM) AllocateIP(key uint64) (net.IP, error) {
	ipam.Lock()
	defer ipam.Unlock()

	if len(ipam.freeIPs) > 0 {
		ip := ipam.freeIPs[0]
		ipam.freeIPs = ipam.freeIPs[1:]
		ipam.busyIPs[key] = ip
		return ip, nil
	} else {
		return nil, errors.New("no free ip available")
	}
}

func (ipam *FTEIDM) AllocateTEID(seID uint64, pdrID uint32) (uint32, error) {
	ipam.Lock()
	defer ipam.Unlock()

	if len(ipam.freeTEIDs) > 0 {
		teid := ipam.freeTEIDs[0]
		ipam.freeTEIDs = ipam.freeTEIDs[1:]
		if _, ok := ipam.busyTEIDs[seID]; !ok {
			pdr := make(map[uint32]uint32)
			pdr[pdrID] = teid
			ipam.busyTEIDs[seID] = pdr
		} else {
			ipam.busyTEIDs[seID][pdrID] = teid
		}
		return teid, nil
	} else {
		return 0, errors.New("no free TEID available")
	}
}

func (ipam *IPAM) ReleaseIP(seID uint64) {
	ipam.Lock()
	defer ipam.Unlock()
	if ip, ok := ipam.busyIPs[seID]; ok {
		ipam.freeIPs = append(ipam.freeIPs, ip)
		delete(ipam.busyIPs, seID)
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
