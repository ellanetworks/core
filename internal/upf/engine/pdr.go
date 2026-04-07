// Copyright 2024 Ella Networks
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
