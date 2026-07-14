// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1
package engine

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func applyPDR(spdrInfo SPDRInfo, sess *Session, bpfObjects *ebpf.BpfObjects) error {
	if spdrInfo.UEIP.IsValid() {
		if err := bpfObjects.PutPdrDownlink(spdrInfo.UEIP, spdrInfo.PdrInfo); err != nil {
			return fmt.Errorf("can't apply downlink PDR: %w", err)
		}

		return nil
	}

	// applyPDR is the sole pdrs_uplink writer, so stamping here covers every apply path.
	v4, v6 := sess.UEAddresses()
	if !v4.IsValid() && !v6.IsValid() {
		logger.UpfLog.Warn("uplink PDR has no UE source address; uplink will be dropped (fail closed)",
			logger.SEID(sess.SEID), logger.TEID(spdrInfo.TeID))
	}

	spdrInfo.PdrInfo.UEIPv4, spdrInfo.PdrInfo.UEIPv6Prefix = v4, v6

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
