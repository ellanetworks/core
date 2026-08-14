// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"testing"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func TestRestampReferencingPDRsLeavesTheDatapathAlone(t *testing.T) {
	eng := newTestEngine()

	sess := addSessionWithPDRs(t, eng, 100, "42")
	if eng.BpfObjects != nil {
		t.Fatal("the fixture has a datapath, so this test cannot detect a write")
	}

	touched := make(map[uint32]struct{})

	far := ebpf.FarInfo{Action: farForward, TeID: 0x7001}

	restampReferencingPDRs(sess, touched, func(SPDRInfo) bool { return true },
		func(p *SPDRInfo) { p.PdrInfo.Far = far })

	qer := ebpf.QerInfo{Qfi: 9}

	restampReferencingPDRs(sess, touched, func(SPDRInfo) bool { return true },
		func(p *SPDRInfo) { p.PdrInfo.Qer = qer })

	if len(touched) != 2 {
		t.Fatalf("touched = %v, want one entry for each of the session's 2 PDRs", touched)
	}

	for _, pdrID := range []uint32{1, 2} {
		if _, ok := touched[pdrID]; !ok {
			t.Errorf("PDR %d was not recorded as needing a datapath write", pdrID)
		}

		got := sess.GetPDR(pdrID).PdrInfo
		if got.Far != far {
			t.Errorf("PDR %d holds FAR %+v, want the merged %+v", pdrID, got.Far, far)
		}

		if got.Qer != qer {
			t.Errorf("PDR %d holds QER %+v, want the merged %+v", pdrID, got.Qer, qer)
		}
	}
}
