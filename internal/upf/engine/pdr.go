// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
package engine

import (
	"fmt"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func applyPDR(spdrInfo SPDRInfo, sess *Session, bpfObjects *ebpf.BpfObjects) error {
	if spdrInfo.UEIP.IsValid() {
		if err := bpfObjects.PutPdrDownlink(spdrInfo.UEIP, spdrInfo.PdrInfo); err != nil {
			return fmt.Errorf("can't apply downlink PDR: %w", err)
		}

		return nil
	}

	// Uplink PDR: stamp the session's authorized source addresses so the
	// datapath can validate the inner source (anti-spoofing). applyPDR is the
	// sole writer of pdrs_uplink, so stamping here covers every apply path.
	spdrInfo.PdrInfo.UEIPv4, spdrInfo.PdrInfo.UEIPv6Prefix = sess.UEAddresses()

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
