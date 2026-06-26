// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/s1ap"
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

	m.removeENB(c1)

	got := m.ListENBs()
	if len(got) != 1 || got[0].Name != "enb-b" {
		t.Fatalf("after remove: %+v", got)
	}

	// Removing an unknown association is a no-op.
	m.removeENB(new(sctp.SCTPConn))

	if got := len(m.ListENBs()); got != 1 {
		t.Fatalf("ListENBs = %d, want 1", got)
	}
}

// TestENBSetupCompleteGate exercises the S1-Setup-complete marker that arms the
// dispatcher's setup-first gate (TS 36.413 §8.7.3.1).
func TestENBSetupCompleteGate(t *testing.T) {
	m := newTestMME(t)
	c := new(sctp.SCTPConn)

	if m.enbSetupComplete(c) {
		t.Fatal("untracked eNB reported setup-complete")
	}

	m.trackENB(c, ENBInfo{Name: "enb-a", ID: "00f110-1"})

	if m.enbSetupComplete(c) {
		t.Fatal("tracked-but-not-set-up eNB reported setup-complete")
	}

	m.markENBSetupComplete(c)

	if !m.enbSetupComplete(c) {
		t.Fatal("eNB not setup-complete after marking")
	}

	m.removeENB(c)

	if m.enbSetupComplete(c) {
		t.Fatal("removed eNB still setup-complete")
	}
}

// TestDispatchGatesUEMessageBeforeS1Setup checks the dispatcher drops UE-
// associated signalling from an association that has not completed S1 Setup
// (TS 36.413 §8.7.3.1): no UE context is created.
func TestDispatchGatesUEMessageBeforeS1Setup(t *testing.T) {
	m := newTestMME(t)

	raw, err := (&s1ap.InitialUEMessage{
		ENBUES1APID:           1,
		NASPDU:                s1ap.NASPDU{0x07, 0x41},
		TAI:                   s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
		EUTRANCGI:             s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		RRCEstablishmentCause: s1ap.RRCCauseMOSignalling,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// nil conn: no S1 Setup has completed on this association, so the message
	// must be dropped before any UE context is created.
	m.dispatch(context.Background(), nil, raw)

	if len(m.conns) != 0 {
		t.Fatalf("UE context created from an Initial UE Message before S1 Setup: %d", len(m.conns))
	}
}
