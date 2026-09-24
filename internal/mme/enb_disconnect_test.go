// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
)

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
	m.FreeUeConn(t.Context(), ue)

	m.reclaimUEsOnConnLoss(cc)

	got, ok := m.LookupUeByIMSI(ue.imsiOrEmpty())
	if !ok || got != ue || got.Connected() {
		t.Fatal("idle UE disturbed by an unrelated eNB disconnect")
	}

	if m.Session.(*fakeSessionManager).deactivated {
		t.Fatal("idle UE's session re-deactivated on eNB disconnect")
	}
}

func TestRepeatedS1SetupKeepsTheUEsUntilItIsAccepted(t *testing.T) {
	m := newTestMME(t)
	conn := new(sctp.SCTPConn)
	now := time.Now()

	m.trackRadio(t.Context(), conn, RadioInfo{Name: "enb-a", ConnectedAt: now, LastSeenAt: now})

	ue := m.NewUe(t.Context(), conn, 7)

	m.trackRadio(t.Context(), conn, RadioInfo{Name: "enb-a", ConnectedAt: now, LastSeenAt: now})

	if !ue.Connected() {
		t.Fatal("receiving a repeated S1 Setup released the UEs before the MME accepted it (TS 36.413 §8.7.3.3)")
	}

	claimENBID(t, m, m.RadioForConn(conn), testENBID(1))

	if ue.Connected() {
		t.Error("an accepted repeated S1 Setup left the UE connected on an association it re-initialised (TS 36.413 §8.7.3.1)")
	}
}

func TestIdleUEKeepsNoENBEndpoint(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.TransitionTo(t.Context(), EMMRegistered)

	m.SetPDNEnbFTEID(ue, testPDN(ue), models.FTEID{TEID: 0x1234, Addr: netip.MustParseAddr("10.0.0.1")})

	m.FreeUeConn(t.Context(), ue)

	for _, p := range m.SnapshotPDNs(ue) {
		if p.EnbFTEID != (models.FTEID{}) {
			t.Fatalf("EBI %d keeps eNB endpoint %+v in ECM-IDLE; the eNB released it with the S1 connection (TS 23.401 §5.3.5)", p.Ebi, p.EnbFTEID)
		}
	}
}
