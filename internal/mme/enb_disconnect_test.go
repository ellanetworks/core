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

	got, ok := m.lookupUe(ue.MMEUES1APID)
	if !ok {
		t.Fatal("registered UE deleted on eNB disconnect; expected ECM-IDLE retention")
	}

	if got.ecmState.load() != ECMIdle {
		t.Fatalf("ecmState = %v, want ECMIdle after eNB disconnect", got.ecmState.load())
	}

	if got.mobileReachableTimer == nil {
		t.Fatal("mobile reachable timer not armed after eNB disconnect")
	}

	if !m.session.(*fakeSessionManager).deactivated {
		t.Fatal("EPS session not deactivated for paging after eNB disconnect")
	}

	m.removeUe(ue.MMEUES1APID) // stop the default-duration timer
}

// TestENBDisconnectDropsMidAttachUE confirms a UE that had not completed
// registration is dropped (and its session released) when its eNB drops.
func TestENBDisconnectDropsMidAttachUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.emmState.store(EMMDeregistered) // attach not yet completed
	testPDN(ue).apn = "internet"

	m.reclaimUEsOnConnLoss(cc)

	if _, ok := m.lookupUe(ue.MMEUES1APID); ok {
		t.Fatal("incomplete-registration UE retained on eNB disconnect; expected drop")
	}

	if !m.session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released when dropping an incomplete UE")
	}
}

// TestENBDisconnectLeavesIdleUE confirms an already-idle UE keeps its own
// supervision and is not disturbed by its eNB's disconnect.
func TestENBDisconnectLeavesIdleUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ecmState.store(ECMIdle) // already idle, own mobile reachable supervision

	m.reclaimUEsOnConnLoss(cc)

	got, ok := m.lookupUe(ue.MMEUES1APID)
	if !ok {
		t.Fatal("idle UE removed on eNB disconnect")
	}

	if got.ecmState.load() != ECMIdle {
		t.Fatalf("idle UE ecmState changed to %v on eNB disconnect", got.ecmState.load())
	}

	if m.session.(*fakeSessionManager).deactivated {
		t.Fatal("idle UE's session re-deactivated on eNB disconnect")
	}
}
