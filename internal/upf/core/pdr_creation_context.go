// Copyright 2024 Ella Networks
package core

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/wmnsk/go-pfcp/ie"
	"go.uber.org/zap"
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
		spdrInfo.PdrInfo.FarID = pdrContext.getFARID(farid)
	}
	if qerid, err := pdr.QERID(); err == nil {
		spdrInfo.PdrInfo.QerID = pdrContext.getQERID(qerid)
	}
	if urrid, err := pdr.URRID(); err == nil {
		spdrInfo.PdrInfo.UrrID = pdrContext.getURRID(urrid)
	}

	pdi, err := pdr.PDI()
	if err != nil {
		return fmt.Errorf("PDI IE is missing: %s", err)
	}

	if sdfFilter, err := pdr.SDFFilter(); err == nil {
		if sdfFilter.FlowDescription == "" {
			logger.UpfLog.Warn("SDFFilter is empty")
		} else if sdfFilterParsed, err := ParseSdfFilter(sdfFilter.FlowDescription); err == nil {
			spdrInfo.PdrInfo.SdfFilter = &sdfFilterParsed
		} else {
			return fmt.Errorf("can't parse SDFFilter: %s", err)
		}
	}

	if teidPdiID := findIEindex(pdi, 21); teidPdiID != -1 { // IE Type F-TEID
		if fteid, err := pdi[teidPdiID].FTEID(); err == nil {
			teid := fteid.TEID
			if fteid.HasCh() {
				allocate := true
				if fteid.HasChID() {
					if teidFromCache, ok := pdrContext.hasTEIDCache(fteid.ChooseID); ok {
						allocate = false
						teid = teidFromCache
						spdrInfo.Allocated = true
						logger.UpfLog.Info("retrieved TEID from cache", zap.Uint32("TEID", teid))
					}
				}
				if allocate {
					allocatedTeID, err := pdrContext.getFTEID(pdrContext.Session.RemoteSEID, spdrInfo.PdrID)
					if err != nil {
						return fmt.Errorf("can't allocate TEID: %s", causeToString(ie.CauseNoResourcesAvailable))
					}
					teid = allocatedTeID
					spdrInfo.Allocated = true
					if fteid.HasChID() {
						pdrContext.setTEIDCache(fteid.ChooseID, teid)
					}
				}
			}
			spdrInfo.TeID = teid
			return nil
		}
		return fmt.Errorf("F-TEID IE is missing")
	} else if ueipPdiID := findIEindex(pdi, 93); ueipPdiID != -1 {
		if ueIP, _ := pdi[ueipPdiID].UEIPAddress(); ueIP != nil {
			if hasCHV4(ueIP.Flags) {
				return fmt.Errorf("UE IP Allocation is not supported in the UPF")
			}
			if ueIP.IPv4Address != nil {
				spdrInfo.Ipv4 = cloneIP(ueIP.IPv4Address)
			} else if ueIP.IPv6Address != nil {
				spdrInfo.Ipv6 = cloneIP(ueIP.IPv6Address)
			} else {
				return fmt.Errorf("UE IP Address IE is missing")
			}
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
		if _, ok := pdrContext.TEIDCache[uint8(spdrInfo.TeID)]; !ok {
			if err := bpfObjects.DeletePdrUplink(spdrInfo.TeID); err != nil {
				return fmt.Errorf("can't delete GTP PDR: %s", err.Error())
			}
			pdrContext.TEIDCache[uint8(spdrInfo.TeID)] = 0
		}
	}
	if spdrInfo.TeID != 0 {
		pdrContext.FteIDResourceManager.ReleaseTEID(pdrContext.Session.RemoteSEID)
	}
	return nil
}

func (pdrContext *PDRCreationContext) getFARID(farid uint32) uint32 {
	return pdrContext.Session.GetFar(farid).GlobalID
}

func (pdrContext *PDRCreationContext) getQERID(qerid uint32) uint32 {
	return pdrContext.Session.GetQer(qerid).GlobalID
}

func (pdrContext *PDRCreationContext) getURRID(urrid uint32) uint32 {
	return pdrContext.Session.GetUrr(urrid)
}

func (pdrContext *PDRCreationContext) getFTEID(seID uint64, pdrID uint32) (uint32, error) {
	if pdrContext.FteIDResourceManager == nil {
		return 0, errors.New("FTEID manager is nil")
	}

	allocatedTeID, err := pdrContext.FteIDResourceManager.AllocateTEID(seID, pdrID)
	if err != nil {
		return 0, fmt.Errorf("can't allocate TEID: %s", causeToString(ie.CauseNoResourcesAvailable))
	}
	return allocatedTeID, nil
}

func (pdrContext *PDRCreationContext) hasTEIDCache(chooseID uint8) (uint32, bool) {
	teid, ok := pdrContext.TEIDCache[chooseID]
	return teid, ok
}

func (pdrContext *PDRCreationContext) setTEIDCache(chooseID uint8, teid uint32) {
	pdrContext.TEIDCache[chooseID] = teid
}

func hasCHV4(flags uint8) bool {
	return flags&(1<<4) != 0
}
