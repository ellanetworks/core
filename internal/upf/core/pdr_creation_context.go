// Copyright 2024 Ella Networks
package core

import (
	"fmt"

	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/wmnsk/go-pfcp/ie"
)

type PDRCreationContext struct {
	Session              *Session
	FteIDResourceManager *FteIDResourceManager
	TEIDCache            map[uint8]uint32
}

func NewPDRCreationContext(session *Session, resourceManager *FteIDResourceManager) *PDRCreationContext {
	return &PDRCreationContext{
		Session:              session,
		FteIDResourceManager: resourceManager,
		TEIDCache:            make(map[uint8]uint32),
	}
}

func (pdrContext *PDRCreationContext) ExtractPDR(pdr *ie.IE, spdrInfo *SPDRInfo) error {
	if outerHeaderRemoval, err := pdr.OuterHeaderRemovalDescription(); err == nil {
		spdrInfo.PdrInfo.OuterHeaderRemoval = outerHeaderRemoval
	}

	if farid, err := pdr.FARID(); err == nil {
		spdrInfo.PdrInfo.FarID = farid
	}

	if qerid, err := pdr.QERID(); err == nil {
		spdrInfo.PdrInfo.QerID = qerid
	}

	if urrid, err := pdr.URRID(); err == nil {
		spdrInfo.PdrInfo.UrrID = urrid
	}

	pdi, err := pdr.PDI()
	if err != nil {
		return fmt.Errorf("PDI IE is missing: %s", err)
	}

	if teidPdiID := findIEindex(pdi, 21); teidPdiID != -1 {
		fteid, err := pdi[teidPdiID].FTEID()
		if err != nil {
			return fmt.Errorf("F-TEID IE is malformed: %s", err)
		}

		if !fteid.HasCh() {
			return fmt.Errorf("only CH flag is supported in F-TEID IE")
		}

		teid, err := pdrContext.allocateTEID()
		if err != nil {
			return fmt.Errorf("can't allocate TEID: %s", causeToString(ie.CauseNoResourcesAvailable))
		}

		spdrInfo.Allocated = true
		spdrInfo.TeID = teid

		return nil
	} else if ueipPdiID := findIEindex(pdi, 93); ueipPdiID != -1 {
		ueIP, _ := pdi[ueipPdiID].UEIPAddress()
		if ueIP == nil {
			return fmt.Errorf("UE IP Address IE is malformed")
		}

		if hasCHV4(ueIP.Flags) {
			return fmt.Errorf("UE IP Allocation is not supported")
		}

		if ueIP.IPv4Address != nil {
			spdrInfo.Ipv4 = cloneIP(ueIP.IPv4Address)
		} else if ueIP.IPv6Address != nil {
			spdrInfo.Ipv6 = cloneIP(ueIP.IPv6Address)
		} else {
			return fmt.Errorf("UE IP Address IE is missing")
		}

		return nil
	} else {
		return fmt.Errorf("both F-TEID IE and UE IP Address IE are missing: %s", err)
	}
}

func (pdrContext *PDRCreationContext) deletePDR(spdrInfo SPDRInfo, bpfObjects *ebpf.BpfObjects) error {
	if spdrInfo.Ipv4 != nil {
		if err := bpfObjects.DeletePdrDownlink(spdrInfo.Ipv4); err != nil {
			return fmt.Errorf("can't delete IPv4 PDR: %s", err.Error())
		}
	} else if spdrInfo.Ipv6 != nil {
		if err := bpfObjects.DeleteDownlinkPdrIP6(spdrInfo.Ipv6); err != nil {
			return fmt.Errorf("can't delete IPv6 PDR: %s", err.Error())
		}
	} else {
		_, ok := pdrContext.TEIDCache[uint8(spdrInfo.TeID)]
		if !ok {
			err := bpfObjects.DeletePdrUplink(spdrInfo.TeID)
			if err != nil {
				return fmt.Errorf("can't delete GTP PDR: %s", err.Error())
			}

			pdrContext.TEIDCache[uint8(spdrInfo.TeID)] = 0
		}
	}

	if spdrInfo.TeID != 0 {
		pdrContext.FteIDResourceManager.ReleaseTEID(pdrContext.Session.SEID)
	}

	return nil
}

func (pdrContext *PDRCreationContext) allocateTEID() (uint32, error) {
	if pdrContext.FteIDResourceManager == nil {
		return 0, fmt.Errorf("FTEID Resource Manager is not initialized")
	}

	allocatedTeID, err := pdrContext.FteIDResourceManager.AllocateTEID(pdrContext.Session.SEID)
	if err != nil {
		return 0, fmt.Errorf("can't allocate TEID: %s", causeToString(ie.CauseNoResourcesAvailable))
	}

	return allocatedTeID, nil
}

func hasCHV4(flags uint8) bool {
	return flags&(1<<4) != 0
}
