// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "testing"

func TestENBDisconnectRetainsRegisteredUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	testPDN(ue).Apn = "internet"

	m.reclaimUEsOnConnLoss(cc)

	got, ok := m.LookupUeByIMSI(ue.imsiOrEmpty())
	if !ok || got != ue {
		t.Fatal("registered UE deleted on eNB disconnect; expected ECM-IDLE retention")
	}

	if got.Connected() {
		t.Fatal("UE not in ECM-IDLE after eNB disconnect")
	}

	if !got.mobileReachableTimer.Active() {
		t.Fatal("mobile reachable timer not armed after eNB disconnect")
	}

	if !m.Session.(*fakeSessionManager).deactivated {
		t.Fatal("EPS session not deactivated for paging after eNB disconnect")
	}

	m.RemoveUe(ue)
}

func TestENBDisconnectDropsMidAttachUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ForceStateForTest(EMMDeregistered)
	testPDN(ue).Apn = "internet"

	m.reclaimUEsOnConnLoss(cc)

	if _, ok := m.LookupUeByIMSI(ue.imsiOrEmpty()); ok {
		t.Fatal("incomplete-registration UE retained on eNB disconnect; expected drop")
	}

	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released when dropping an incomplete UE")
	}
}

func TestENBDisconnectLeavesIdleUE(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	m.FreeUeConn(ue)

	m.reclaimUEsOnConnLoss(cc)

	got, ok := m.LookupUeByIMSI(ue.imsiOrEmpty())
	if !ok || got != ue || got.Connected() {
		t.Fatal("idle UE disturbed by an unrelated eNB disconnect")
	}

	if m.Session.(*fakeSessionManager).deactivated {
		t.Fatal("idle UE's session re-deactivated on eNB disconnect")
	}
}
