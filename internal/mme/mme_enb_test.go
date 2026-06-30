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

	m.trackENB(c1, ENBInfo{Name: "enb-a", ID: "00f110-1"})
	m.trackENB(c2, ENBInfo{Name: "enb-b", ID: "00f110-2"})

	if got := len(m.ListENBs()); got != 2 {
		t.Fatalf("ListENBs = %d, want 2", got)
	}

	m.RemoveENB(c1)

	got := m.ListENBs()
	if len(got) != 1 || got[0].Name != "enb-b" {
		t.Fatalf("after remove: %+v", got)
	}

	// Removing an unknown association is a no-op.
	m.RemoveENB(new(sctp.SCTPConn))

	if got := len(m.ListENBs()); got != 1 {
		t.Fatalf("ListENBs = %d, want 1", got)
	}
}

// TestENBSetupCompleteGate exercises the S1-Setup-complete marker that arms the
// dispatcher's setup-first gate (TS 36.413 §8.7.3.1).
func TestENBSetupCompleteGate(t *testing.T) {
	m := newTestMME(t)
	c := new(sctp.SCTPConn)

	if m.ENBSetupComplete(c) {
		t.Fatal("untracked eNB reported setup-complete")
	}

	m.trackENB(c, ENBInfo{Name: "enb-a", ID: "00f110-1"})

	if m.ENBSetupComplete(c) {
		t.Fatal("tracked-but-not-set-up eNB reported setup-complete")
	}

	m.MarkENBSetupComplete(c)

	if !m.ENBSetupComplete(c) {
		t.Fatal("eNB not setup-complete after marking")
	}

	m.RemoveENB(c)

	if m.ENBSetupComplete(c) {
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

	m.trackENB(c1, ENBInfo{Name: "enb-old", ID: id})
	m.MarkENBSetupComplete(c1)

	if got := m.enbByID[id]; got != NasWriter(c1) {
		t.Fatalf("setup: enbByID[%q] resolved to the wrong association", id)
	}

	m.trackENB(c2, ENBInfo{Name: "enb-new", ID: id})
	m.MarkENBSetupComplete(c2)

	if got := m.enbByID[id]; got != NasWriter(c2) {
		t.Errorf("enbByID[%q] did not resolve to the re-associated eNB", id)
	}

	if m.ENBSetupComplete(c1) {
		t.Error("stale association should have been evicted from the eNB table")
	}

	if !m.ENBSetupComplete(c2) {
		t.Error("current association should be setup-complete")
	}

	if got := m.ListENBs(); len(got) != 1 || got[0].Name != "enb-new" {
		t.Errorf("ListENBs = %+v, want only enb-new", got)
	}
}
