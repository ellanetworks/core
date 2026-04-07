// Copyright 2024 Ella Networks
package engine

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
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

func (pdrContext *PDRCreationContext) deletePDR(spdrInfo SPDRInfo, bpfObjects *ebpf.BpfObjects) error {
	if spdrInfo.UEIP.IsValid() {
		if err := bpfObjects.DeletePdrDownlink(spdrInfo.UEIP); err != nil {
			return fmt.Errorf("can't delete downlink PDR: %s", err.Error())
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

	if err := bpfObjects.DeleteUrr(spdrInfo.PdrInfo.UrrID); err != nil {
		return fmt.Errorf("could not delete URR %d: %s", spdrInfo.PdrInfo.UrrID, err)
	}

	return nil
}

func (pdrContext *PDRCreationContext) allocateTEID() (uint32, error) {
	if pdrContext.FteIDResourceManager == nil {
		return 0, fmt.Errorf("FTEID Resource Manager is not initialized")
	}

	allocatedTeID, err := pdrContext.FteIDResourceManager.AllocateTEID(pdrContext.Session.SEID)
	if err != nil {
		return 0, fmt.Errorf("can't allocate TEID: no resources available")
	}

	return allocatedTeID, nil
}

// ExtractPDR populates spdrInfo from a models.PDR, looking up
// the referenced FAR and QER from the provided maps.
func (pdrContext *PDRCreationContext) ExtractPDR(pdr models.PDR, spdrInfo *SPDRInfo, farMap map[uint32]ebpf.FarInfo, qerMap map[uint32]ebpf.QerInfo) error {
	if pdr.OuterHeaderRemoval != nil {
		spdrInfo.PdrInfo.OuterHeaderRemoval = *pdr.OuterHeaderRemoval
	}

	spdrInfo.PdrInfo.FarID = pdr.FARID
	spdrInfo.PdrInfo.Far = farMap[pdr.FARID]

	spdrInfo.PdrInfo.QerID = pdr.QERID
	spdrInfo.PdrInfo.Qer = qerMap[pdr.QERID]

	spdrInfo.PdrInfo.UrrID = pdr.URRID

	if pdr.PDI.LocalFTEID != nil {
		teid, err := pdrContext.allocateTEID()
		if err != nil {
			return fmt.Errorf("can't allocate TEID: %w", err)
		}

		spdrInfo.Allocated = true
		spdrInfo.TeID = teid

		return nil
	}

	if pdr.PDI.UEIPAddress.IsValid() {
		spdrInfo.UEIP = pdr.PDI.UEIPAddress

		return nil
	}

	return fmt.Errorf("both F-TEID and UE IP Address are missing")
}
