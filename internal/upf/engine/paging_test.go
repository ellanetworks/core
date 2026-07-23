// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"testing"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

// TestSuppressDownlinkDataNotification_KeepsDownlinkDeduped reproduces the paging
// storm of issue #1493 at the datapath: an unreachable idle UE was re-paged by
// every downlink packet because the paging-failure path cleared the notify-once
// dedup. Suppression keeps the downlink detection rule deduped so no further page
// is triggered until the session is reactivated (TS 23.401 §5.3.4.3; TS 23.502
// §4.2.3.3). The uplink rule never produces a notification and is left untouched.
func TestSuppressDownlinkDataNotification_KeepsDownlinkDeduped(t *testing.T) {
	eng := newTestEngine()
	eng.BpfObjects = ebpf.NewBpfObjects(false, false, 1, 0, 0, 0)

	const seid = uint64(1)

	addSessionWithPDRs(t, eng, seid, "policy")

	downlink := ebpf.DataNotification{LocalSEID: seid, PdrID: 2, QFI: 0}
	uplink := ebpf.DataNotification{LocalSEID: seid, PdrID: 1, QFI: 0}

	eng.SuppressDownlinkDataNotification(seid)

	if !eng.BpfObjects.IsAlreadyNotified(downlink) {
		t.Fatal("downlink notification not suppressed after paging failure: an unreachable UE would be re-paged by the next packet")
	}

	if eng.BpfObjects.IsAlreadyNotified(uplink) {
		t.Fatal("uplink rule marked notified: suppression must target downlink detection rules only")
	}

	// Reactivation clears the dedup, so paging recovers when the UE returns.
	eng.BpfObjects.ClearNotified(seid, 2, 0)

	if eng.BpfObjects.IsAlreadyNotified(downlink) {
		t.Fatal("downlink notification still suppressed after reactivation: UE would never be paged again")
	}
}
