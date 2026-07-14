// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
package engine

import (
	"fmt"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func applyPDR(spdrInfo SPDRInfo, bpfObjects *ebpf.BpfObjects) error {
	if spdrInfo.UEIP.IsValid() {
		if err := bpfObjects.PutPdrDownlink(spdrInfo.UEIP, spdrInfo.PdrInfo); err != nil {
			return fmt.Errorf("can't apply downlink PDR: %w", err)
		}

		return nil
	}

	if err := bpfObjects.PutPdrUplink(spdrInfo.TeID, spdrInfo.PdrInfo); err != nil {
		return fmt.Errorf("can't apply GTP PDR: %w", err)
	}

	return nil
}

// unapplyPDR removes the eBPF map entry applyPDR installed for spdrInfo.
func unapplyPDR(spdrInfo SPDRInfo, bpfObjects *ebpf.BpfObjects) error {
	if spdrInfo.UEIP.IsValid() {
		return bpfObjects.DeletePdrDownlink(spdrInfo.UEIP)
	}

	if spdrInfo.TeID != 0 {
		return bpfObjects.DeletePdrUplink(spdrInfo.TeID)
	}

	return nil
}
