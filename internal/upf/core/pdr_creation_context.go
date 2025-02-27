// Copyright 2024 Ella Networks
package core

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/config"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/wmnsk/go-pfcp/ie"
)

type PDRCreationContext struct {
	Session         *Session
	ResourceManager *ResourceManager
	TEIDCache       map[uint8]uint32
}

func NewPDRCreationContext(session *Session, resourceManager *ResourceManager) *PDRCreationContext {
	return &PDRCreationContext{
		Session:         session,
		ResourceManager: resourceManager,
		TEIDCache:       make(map[uint8]uint32),
	}
}

func (pdrContext *PDRCreationContext) ExtractPDR(pdr *ie.IE, spdrInfo *SPDRInfo) error {
	if outerHeaderRemoval, err := pdr.OuterHeaderRemovalDescription(); err == nil {
		spdrInfo.PdrInfo.OuterHeaderRemoval = outerHeaderRemoval
	}
	if farid, err := pdr.FARID(); err == nil {
		spdrInfo.PdrInfo.FarId = pdrContext.getFARID(farid)
	}
	if qerid, err := pdr.QERID(); err == nil {
		spdrInfo.PdrInfo.QerId = pdrContext.getQERID(qerid)
	}

	pdi, err := pdr.PDI()
	if err != nil {
		return fmt.Errorf("PDI IE is missing")
	}

	if sdfFilter, err := pdr.SDFFilter(); err == nil {
		if sdfFilter.FlowDescription == "" {
			logger.UpfLog.Warnf("SDFFilter is empty")
		} else if sdfFilterParsed, err := ParseSdfFilter(sdfFilter.FlowDescription); err == nil {
			spdrInfo.PdrInfo.SdfFilter = &sdfFilterParsed
		} else {
			logger.UpfLog.Errorf("SDFFilter err: %v", err)
			return err
		}
	}

	if teidPdiId := findIEindex(pdi, 21); teidPdiId != -1 { // IE Type F-TEID
		if fteid, err := pdi[teidPdiId].FTEID(); err == nil {
			teid := fteid.TEID
			if fteid.HasCh() {
				allocate := true
				if fteid.HasChID() {
					if teidFromCache, ok := pdrContext.hasTEIDCache(fteid.ChooseID); ok {
						allocate = false
						teid = teidFromCache
						spdrInfo.Allocated = true
						logger.UpfLog.Infof("TEID from cache: %d", teid)
					}
				}
				if allocate {
					allocatedTeid, err := pdrContext.getFTEID(pdrContext.Session.RemoteSEID, spdrInfo.PdrID)
					if err != nil {
						return fmt.Errorf("can't allocate TEID: %s", causeToString(ie.CauseNoResourcesAvailable))
					}
					teid = allocatedTeid
					spdrInfo.Allocated = true
					if fteid.HasChID() {
						pdrContext.setTEIDCache(fteid.ChooseID, teid)
					}
				}
			}
			spdrInfo.Teid = teid
			return nil
		}
		return fmt.Errorf("F-TEID IE is missing")
	} else if ueipPdiId := findIEindex(pdi, 93); ueipPdiId != -1 {
		if ueIp, _ := pdi[ueipPdiId].UEIPAddress(); ueIp != nil {
			if config.Conf.FeatureUEIP && hasCHV4(ueIp.Flags) {
				return fmt.Errorf("UE IP Allocation is not supported in the UPF")
			}
			if ueIp.IPv4Address != nil {
				spdrInfo.Ipv4 = cloneIP(ueIp.IPv4Address)
			} else if ueIp.IPv6Address != nil {
				spdrInfo.Ipv6 = cloneIP(ueIp.IPv6Address)
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
		if err := bpfObjects.DeleteDownlinkPdrIp6(spdrInfo.Ipv6); err != nil {
			return fmt.Errorf("can't delete IPv6 PDR: %s", err.Error())
		}
	} else {
		if _, ok := pdrContext.TEIDCache[uint8(spdrInfo.Teid)]; !ok {
			if err := bpfObjects.DeletePdrUplink(spdrInfo.Teid); err != nil {
				return fmt.Errorf("can't delete GTP PDR: %s", err.Error())
			}
			pdrContext.TEIDCache[uint8(spdrInfo.Teid)] = 0
		}
	}
	if spdrInfo.Teid != 0 {
		pdrContext.ResourceManager.FTEIDM.ReleaseTEID(pdrContext.Session.RemoteSEID)
	}
	return nil
}

func (pdrContext *PDRCreationContext) getFARID(farid uint32) uint32 {
	return pdrContext.Session.GetFar(farid).GlobalId
}

func (pdrContext *PDRCreationContext) getQERID(qerid uint32) uint32 {
	return pdrContext.Session.GetQer(qerid).GlobalId
}

func (pdrContext *PDRCreationContext) getFTEID(seID uint64, pdrID uint32) (uint32, error) {
	if pdrContext.ResourceManager == nil || pdrContext.ResourceManager.FTEIDM == nil {
		return 0, errors.New("FTEID manager is nil")
	}

	allocatedTeid, err := pdrContext.ResourceManager.FTEIDM.AllocateTEID(seID, pdrID)
	if err != nil {
		return 0, fmt.Errorf("can't allocate TEID: %s", causeToString(ie.CauseNoResourcesAvailable))
	}
	return allocatedTeid, nil
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
