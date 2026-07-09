// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
)

// testENBID builds a Global eNB ID rendering to "00f110-<value>" via ENBID.
func testENBID(value uint32) s1ap.GlobalENBID {
	return s1ap.GlobalENBID{
		PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10},
		ENBID:        s1ap.ENBID{Value: value},
	}
}

func TestENBTable(t *testing.T) {
	m := newTestMME(t)

	c1 := new(sctp.SCTPConn)
	c2 := new(sctp.SCTPConn)

	m.trackRadio(c1, RadioInfo{Name: "enb-a", ID: "00f110-1"})
	m.trackRadio(c2, RadioInfo{Name: "enb-b", ID: "00f110-2"})

	if got := len(m.ListRadios()); got != 2 {
		t.Fatalf("ListRadios = %d, want 2", got)
	}

	m.RemoveRadio(c1)

	got := m.ListRadios()
	if len(got) != 1 || got[0].Name != "enb-b" {
		t.Fatalf("after remove: %+v", got)
	}

	// Removing an unknown association is a no-op.
	m.RemoveRadio(new(sctp.SCTPConn))

	if got := len(m.ListRadios()); got != 1 {
		t.Fatalf("ListRadios = %d, want 1", got)
	}
}

// TestENBSetupCompleteGate exercises the S1-Setup-complete marker that arms the
// dispatcher's setup-first gate (TS 36.413 §8.7.3.1).
func TestENBSetupCompleteGate(t *testing.T) {
	m := newTestMME(t)
	c := new(sctp.SCTPConn)

	setupComplete := func(conn *sctp.SCTPConn) bool {
		r := m.RadioForConn(conn)
		return r != nil && r.SetupComplete()
	}

	if setupComplete(c) {
		t.Fatal("untracked eNB reported setup-complete")
	}

	m.trackRadio(c, RadioInfo{Name: "enb-a"})

	if setupComplete(c) {
		t.Fatal("tracked-but-not-set-up eNB reported setup-complete")
	}

	m.ClaimENBID(m.RadioForConn(c), testENBID(1))

	if !setupComplete(c) {
		t.Fatal("eNB not setup-complete after claiming its Global eNB ID")
	}

	m.RemoveRadio(c)

	if setupComplete(c) {
		t.Fatal("removed eNB still setup-complete")
	}
}

// TestClaimENBID_EvictsStaleReassociation asserts an eNB re-associating and completing
// S1 Setup under a Global eNB ID still held by a live association evicts the stale one,
// so the ID resolves to the current association (mirrors the AMF's ClaimRanID eviction).
func TestClaimENBID_EvictsStaleReassociation(t *testing.T) {
	m := newTestMME(t)

	enbID := testENBID(1)
	id := ENBID(enbID)

	c1 := new(sctp.SCTPConn)
	c2 := new(sctp.SCTPConn)

	m.trackRadio(c1, RadioInfo{Name: "enb-old"})
	m.ClaimENBID(m.RadioForConn(c1), enbID)

	if got := m.radiosByID[id]; got == nil || got.Conn != S1APWriter(c1) {
		t.Fatalf("setup: radiosByID[%q] resolved to the wrong association", id)
	}

	m.trackRadio(c2, RadioInfo{Name: "enb-new"})
	m.ClaimENBID(m.RadioForConn(c2), enbID)

	if got := m.radiosByID[id]; got == nil || got.Conn != S1APWriter(c2) {
		t.Errorf("radiosByID[%q] did not resolve to the re-associated eNB", id)
	}

	if r := m.RadioForConn(c1); r != nil && r.SetupComplete() {
		t.Error("stale association should have been evicted from the eNB table")
	}

	if r := m.RadioForConn(c2); r == nil || !r.SetupComplete() {
		t.Error("current association should be setup-complete")
	}

	if got := m.ListRadios(); len(got) != 1 || got[0].Name != "enb-new" {
		t.Errorf("ListRadios = %+v, want only enb-new", got)
	}
}
