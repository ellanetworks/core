// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"testing"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func TestSuppressDownlinkDataNotification_KeepsDownlinkDeduped(t *testing.T) {
	eng := newTestEngine()
	eng.BpfObjects = ebpf.NewBpfObjects(false, false, 1, 0, 0, 0)

	const seid = uint64(1)

	addSessionWithPDRs(t, eng, seid, "policy")

	downlink := ebpf.DataNotification{LocalSEID: seid, PdrID: 2, QFI: 0}
	uplink := ebpf.DataNotification{LocalSEID: seid, PdrID: 1, QFI: 0}

	eng.SuppressDownlinkDataNotification(seid)

	if !eng.BpfObjects.IsAlreadyNotified(downlink) {
		t.Fatal("downlink notification not suppressed")
	}

	if eng.BpfObjects.IsAlreadyNotified(uplink) {
		t.Fatal("uplink rule marked notified, want downlink only")
	}

	eng.BpfObjects.ClearNotified(seid, 2, 0)

	if eng.BpfObjects.IsAlreadyNotified(downlink) {
		t.Fatal("downlink notification still suppressed after ClearNotified")
	}
}

func TestClearDownlinkDataNotification_ReleasesSuppression(t *testing.T) {
	eng := newTestEngine()
	eng.BpfObjects = ebpf.NewBpfObjects(false, false, 1, 0, 0, 0)

	const seid = uint64(1)

	addSessionWithPDRs(t, eng, seid, "policy")

	downlink := ebpf.DataNotification{LocalSEID: seid, PdrID: 2, QFI: 0}

	eng.SuppressDownlinkDataNotification(seid)

	if !eng.BpfObjects.IsAlreadyNotified(downlink) {
		t.Fatal("downlink notification not suppressed")
	}

	eng.ClearDownlinkDataNotification(seid)

	if eng.BpfObjects.IsAlreadyNotified(downlink) {
		t.Fatal("downlink notification still suppressed after clear")
	}
}
