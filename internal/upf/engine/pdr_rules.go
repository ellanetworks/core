// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
package engine

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

// deletePDR removes the datapath entry a PDR installed and returns its local
// F-TEID to the pool. The session's URRs outlive its PDRs and are converged on
// their own.
func deletePDR(seid uint64, spdrInfo SPDRInfo, bpfObjects *ebpf.BpfObjects, resourceManager *FteIDResourceManager) error {
	if spdrInfo.UEIP.IsValid() {
		if err := bpfObjects.DeletePdrDownlink(spdrInfo.UEIP); err != nil {
			return fmt.Errorf("can't delete downlink PDR: %w", err)
		}
	} else if spdrInfo.TeID != 0 {
		if err := bpfObjects.DeletePdrUplink(spdrInfo.TeID); err != nil {
			return fmt.Errorf("can't delete GTP PDR: %w", err)
		}
	}

	if spdrInfo.TeID != 0 && resourceManager != nil {
		resourceManager.ReleaseTEID(seid, spdrInfo.TeID)
	}

	return nil
}

// extractPDR resolves a stated models.PDR into the datapath rule it describes,
// binding the FAR and QER it names. An uplink PDR without a local F-TEID draws
// one from allocTEID.
func extractPDR(pdr models.PDR, spdrInfo *SPDRInfo, farMap map[uint32]ebpf.FarInfo, qerMap map[uint32]ebpf.QerInfo, allocTEID func() (uint32, error)) error {
	if pdr.OuterHeaderRemoval != nil {
		spdrInfo.PdrInfo.OuterHeaderRemoval = *pdr.OuterHeaderRemoval
	} else {
		spdrInfo.PdrInfo.OuterHeaderRemoval = 0
	}

	spdrInfo.PdrInfo.FarID = pdr.FARID
	spdrInfo.PdrInfo.Far = farMap[pdr.FARID]

	spdrInfo.PdrInfo.QerID = pdr.QERID
	spdrInfo.PdrInfo.Qer = qerMap[pdr.QERID]

	spdrInfo.PdrInfo.UrrID = pdr.URRID

	if pdr.PDI.LocalFTEID != nil {
		spdrInfo.UEIP = netip.Addr{}

		if spdrInfo.TeID == 0 {
			teid, err := allocTEID()
			if err != nil {
				return fmt.Errorf("can't allocate TEID: %w", err)
			}

			spdrInfo.Allocated = true
			spdrInfo.TeID = teid
		}

		return nil
	}

	if pdr.PDI.UEIPAddress.IsValid() {
		spdrInfo.UEIP = pdr.PDI.UEIPAddress

		return nil
	}

	return fmt.Errorf("both F-TEID and UE IP Address are missing")
}
