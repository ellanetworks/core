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
