// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "testing"

// TestENBDisconnectRetainsRegisteredUE confirms a registered UE whose eNB
// association drops is retained in ECM-IDLE under mobile reachable supervision.
func TestENBDisconnectRetainsRegisteredUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	testPDN(ue).apn = "internet"

	m.reclaimUEsOnConnLoss(cc)

	got, ok := m.lookupUeByIMSI(ue.imsi)
	if !ok || got != ue {
		t.Fatal("registered UE deleted on eNB disconnect; expected ECM-IDLE retention")
	}

	if got.connected() {
		t.Fatal("UE not in ECM-IDLE after eNB disconnect")
	}

	if got.mobileReachableTimer == nil {
		t.Fatal("mobile reachable timer not armed after eNB disconnect")
	}

	if !m.session.(*fakeSessionManager).deactivated {
		t.Fatal("EPS session not deactivated for paging after eNB disconnect")
	}

	m.removeUe(ue) // stop the default-duration timer
}

// TestENBDisconnectDropsMidAttachUE confirms a UE that had not completed
// registration is dropped (and its session released) when its eNB drops.
func TestENBDisconnectDropsMidAttachUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.emmState.store(EMMDeregistered) // attach not yet completed
	testPDN(ue).apn = "internet"

	m.reclaimUEsOnConnLoss(cc)

	if _, ok := m.lookupUeByIMSI(ue.imsi); ok {
		t.Fatal("incomplete-registration UE retained on eNB disconnect; expected drop")
	}

	if !m.session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released when dropping an incomplete UE")
	}
}

// TestENBDisconnectLeavesIdleUE confirms an already-idle UE on no association is
// not disturbed by an eNB's disconnect.
func TestENBDisconnectLeavesIdleUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	m.freeS1Conn(ue) // already idle

	m.reclaimUEsOnConnLoss(cc)

	got, ok := m.lookupUeByIMSI(ue.imsi)
	if !ok || got != ue || got.connected() {
		t.Fatal("idle UE disturbed by an unrelated eNB disconnect")
	}

	if m.session.(*fakeSessionManager).deactivated {
		t.Fatal("idle UE's session re-deactivated on eNB disconnect")
	}
}
