// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"fmt"
	"slices"

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

func (pdrContext *PDRCreationContext) deletePDR(spdrInfo SPDRInfo, bpfObjects *ebpf.BpfObjects, deleteURR bool) error {
	switch {
	case sdfDownlink(spdrInfo):
	case spdrInfo.UEIP.IsValid():
		if err := bpfObjects.DeletePdrDownlink(spdrInfo.UEIP); err != nil {
			return fmt.Errorf("can't delete downlink PDR: %s", err.Error())
		}
	case spdrInfo.TeID != 0:
		if err := unapplyPDR(spdrInfo, pdrContext.Session, bpfObjects); err != nil {
			return fmt.Errorf("can't delete GTP PDR: %s", err.Error())
		}
	}

	if !deleteURR {
		return nil
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

func forwardsBetweenAccess(pdr models.PDR, destinations map[uint32]models.Interface) bool {
	destination, ok := destinations[pdr.FARID]

	return ok && pdr.PDI.SourceInterface == models.InterfaceAccess && destination == models.InterfaceAccess
}

func farDestinations(fars []models.FAR) map[uint32]models.Interface {
	out := make(map[uint32]models.Interface, len(fars))

	for _, far := range fars {
		if far.ForwardingParameters == nil {
			continue
		}

		out[far.FARID] = far.ForwardingParameters.DestinationInterface
	}

	return out
}

func (pdrContext *PDRCreationContext) ExtractPDR(pdr models.PDR, spdrInfo *SPDRInfo, farMap map[uint32]ebpf.FarInfo, destinations map[uint32]models.Interface, qerMap map[uint32]ebpf.QerInfo) (allocated bool, err error) {
	if pdr.OuterHeaderRemoval != nil {
		spdrInfo.PdrInfo.OuterHeaderRemoval = *pdr.OuterHeaderRemoval
	}

	spdrInfo.PdrInfo.FarID = pdr.FARID
	spdrInfo.PdrInfo.Far = farMap[pdr.FARID]
	spdrInfo.PdrInfo.Forwarding = forwardsBetweenAccess(pdr, destinations)

	spdrInfo.PdrInfo.QerID = pdr.QERID
	spdrInfo.PdrInfo.Qer = qerMap[pdr.QERID]

	spdrInfo.PdrInfo.UrrID = pdr.URRID

	spdrInfo.PdrInfo.QFI = pdr.PDI.QFI
	spdrInfo.Precedence = pdr.Precedence
	spdrInfo.SDF = slices.Clone(pdr.PDI.SDFFilters)

	if pdr.PDI.LocalFTEID != nil {
		if spdrInfo.ChooseID != pdr.PDI.LocalFTEID.ChooseID {
			spdrInfo.TeID = 0
		}

		spdrInfo.ChooseID = pdr.PDI.LocalFTEID.ChooseID

		if spdrInfo.TeID != 0 {
			return false, nil
		}

		if spdrInfo.ChooseID != 0 {
			if teid := pdrContext.Session.chosenTEID(spdrInfo.ChooseID); teid != 0 {
				spdrInfo.TeID = teid
				return false, nil
			}
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
