// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/sctp"
)

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

	m.trackRadio(c, RadioInfo{Name: "enb-a", ID: "00f110-1"})

	if setupComplete(c) {
		t.Fatal("tracked-but-not-set-up eNB reported setup-complete")
	}

	m.MarkRadioSetupComplete(c)

	if !setupComplete(c) {
		t.Fatal("eNB not setup-complete after marking")
	}

	m.RemoveRadio(c)

	if setupComplete(c) {
		t.Fatal("removed eNB still setup-complete")
	}
}

// TestMarkENBSetupComplete_EvictsStaleReassociation asserts that when an eNB
// re-associates and completes S1 Setup under a Global eNB ID still held by an
// earlier live association, the stale association is evicted so the ID resolves
// to the current one (mirrors the AMF's ClaimRanID eviction).
func TestMarkENBSetupComplete_EvictsStaleReassociation(t *testing.T) {
	m := newTestMME(t)

	const id = "00f110-1"

	c1 := new(sctp.SCTPConn)
	c2 := new(sctp.SCTPConn)

	m.trackRadio(c1, RadioInfo{Name: "enb-old", ID: id})
	m.MarkRadioSetupComplete(c1)

	if got := m.radiosByID[id]; got == nil || got.Conn != S1APWriter(c1) {
		t.Fatalf("setup: radiosByID[%q] resolved to the wrong association", id)
	}

	m.trackRadio(c2, RadioInfo{Name: "enb-new", ID: id})
	m.MarkRadioSetupComplete(c2)

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
