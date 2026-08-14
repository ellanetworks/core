// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

type PDRCreationContext struct {
	Session              *Session
	FteIDResourceManager *FteIDResourceManager
}

func NewPDRCreationContext(session *Session, resourceManager *FteIDResourceManager) *PDRCreationContext {
	return &PDRCreationContext{
		Session:              session,
		FteIDResourceManager: resourceManager,
	}
}

func (pdrContext *PDRCreationContext) deletePDR(spdrInfo SPDRInfo, bpfObjects *ebpf.BpfObjects) error {
	if spdrInfo.UEIP.IsValid() {
		if err := bpfObjects.DeletePdrDownlink(spdrInfo.UEIP); err != nil {
			return fmt.Errorf("can't delete downlink PDR: %s", err.Error())
		}
	} else if spdrInfo.TeID != 0 {
		if err := bpfObjects.DeletePdrUplink(spdrInfo.TeID); err != nil {
			return fmt.Errorf("can't delete GTP PDR: %s", err.Error())
		}
	}

	if spdrInfo.TeID != 0 {
		pdrContext.FteIDResourceManager.ReleaseTEID(pdrContext.Session.SEID, spdrInfo.TeID)
	}

	if err := bpfObjects.DeleteUrr(pdrContext.Session.SEID, spdrInfo.PdrInfo.UrrID); err != nil {
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

func (pdrContext *PDRCreationContext) ExtractPDR(pdr models.PDR, spdrInfo *SPDRInfo, farMap map[uint32]ebpf.FarInfo, qerMap map[uint32]ebpf.QerInfo) (allocated bool, err error) {
	if pdr.OuterHeaderRemoval != nil {
		spdrInfo.PdrInfo.OuterHeaderRemoval = *pdr.OuterHeaderRemoval
	}

	spdrInfo.PdrInfo.FarID = pdr.FARID
	spdrInfo.PdrInfo.Far = farMap[pdr.FARID]

	spdrInfo.PdrInfo.QerID = pdr.QERID
	spdrInfo.PdrInfo.Qer = qerMap[pdr.QERID]

	spdrInfo.PdrInfo.UrrID = pdr.URRID

	if pdr.PDI.LocalFTEID != nil {
		if spdrInfo.TeID != 0 {
			return false, nil
		}

		teid, err := pdrContext.allocateTEID()
		if err != nil {
			return false, fmt.Errorf("can't allocate TEID: %w", err)
		}

		spdrInfo.TeID = teid

		return true, nil
	}

	if pdr.PDI.UEIPAddress.IsValid() {
		spdrInfo.UEIP = pdr.PDI.UEIPAddress

		return false, nil
	}

	return false, fmt.Errorf("both F-TEID and UE IP Address are missing")
}
