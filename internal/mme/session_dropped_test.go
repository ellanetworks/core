// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
)

// TS 23.502 §4.11.2.3 step 10
func TestSessionDropped(t *testing.T) {
	t.Run("not the last PDN: nothing is signalled and the UE stays attached", func(t *testing.T) {
		m := newTestMME(t)
		ue, cc := securedUE(t, m)

		p := testPDN(ue)
		p.SessionRef = "imsi-001010000000001-3#1"

		ue.EnsurePDN(6)

		sent := len(cc.sent)

		m.SessionDropped(context.Background(), ue.IMSI(), DefaultERABID, p.SessionRef)

		if m.LookupPDN(ue, DefaultERABID) != nil {
			t.Error("the moved PDN connection was not dropped")
		}

		if m.LookupPDN(ue, 6) == nil {
			t.Error("a PDN connection that did not move was dropped")
		}

		if len(cc.sent) != sent {
			t.Errorf("sent %d S1AP messages, want 0: step 10 excludes the S1AP leg", len(cc.sent)-sent)
		}

		if ue.EMMState() == EMMDeregistered {
			t.Error("the UE was detached though it still holds a PDN connection")
		}
	})

	t.Run("the last PDN: the context waits for the acknowledgement", func(t *testing.T) {
		m := newTestMME(t)
		ue, _ := securedUE(t, m)
		ue.TransitionTo(EMMRegistered)

		p := testPDN(ue)
		p.SessionRef = "imsi-001010000000001-3#1"

		m.SessionDropped(context.Background(), ue.IMSI(), DefaultERABID, p.SessionRef)

		if m.LookupPDN(ue, DefaultERABID) != nil {
			t.Error("the moved PDN connection was not dropped")
		}

		if ue.EMMState() == EMMDeregistered {
			t.Fatal("the context was released before the acknowledgement, which now has nothing to acknowledge")
		}

		if _, ok := m.LookupUeBySupi(ue.Supi()); !ok {
			t.Fatal("the context is unreachable by SUPI, so MMContextAck cannot find it")
		}
	})

	t.Run("a stale ref names another instance and is ignored", func(t *testing.T) {
		m := newTestMME(t)
		ue, _ := securedUE(t, m)

		p := testPDN(ue)
		p.SessionRef = "imsi-001010000000001-3#2"

		m.SessionDropped(context.Background(), ue.IMSI(), DefaultERABID, "imsi-001010000000001-3#1")

		if m.LookupPDN(ue, DefaultERABID) == nil {
			t.Error("a report for a superseded session dropped the current one")
		}
	})
}

// TS 23.401 §5.4.4.1
func TestReleasePDNDeregistersAUELeftWithNone(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.TransitionTo(EMMRegistered)

	p := testPDN(ue)
	p.SessionRef = "imsi-001010000000001-3#1"

	m.ReleasePDN(context.Background(), ue, p)

	if ue.PDNCount() != 0 {
		t.Fatalf("PDN connections = %d, want 0", ue.PDNCount())
	}

	if ue.EMMState() != EMMDeregistered {
		t.Error("a UE left with no PDN connection is still EMM-REGISTERED")
	}
}

// TS 23.401 §5.10.3
func TestReleasePDNKeepsAUEWithAnotherConnection(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.TransitionTo(EMMRegistered)

	p := testPDN(ue)
	p.SessionRef = "imsi-001010000000001-3#1"

	ue.EnsurePDN(6)

	m.ReleasePDN(context.Background(), ue, p)

	if ue.EMMState() == EMMDeregistered {
		t.Error("the UE was detached though it still holds a PDN connection")
	}
}

// TS 23.502 §4.11.1.3.3
func TestMMContextAckReleasesTheContextOfAUEThatLeftEUTRAN(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.TransitionTo(EMMRegistered)

	p := testPDN(ue)
	p.SessionRef = "imsi-001010000000001-3#1"

	m.mu.Lock()
	m.detachConnLocked(ue)
	m.mu.Unlock()

	m.SessionDropped(context.Background(), ue.IMSI(), DefaultERABID, p.SessionRef)

	if _, ok := m.LookupUeByIMSI(ue.IMSI()); !ok {
		t.Fatal("the context was released before the acknowledgement could reach it")
	}

	if err := m.MMContextAck(context.Background(), ue.Supi(), []uint8{p.PDUSessionID}); err != nil {
		t.Fatalf("MMContextAck: %v", err)
	}

	if _, ok := m.LookupUeByIMSI(ue.IMSI()); ok {
		t.Error("the UE context outlived its last PDN connection: it has left E-UTRAN, so nothing else will ever release it and its M-TMSI stays allocated")
	}
}

// TS 23.502 §4.11.1.3.3
func TestMMContextAckReleasesPDNsFiveGSDidNotAdopt(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.TransitionTo(EMMRegistered)

	moved := testPDN(ue)
	moved.SessionRef = "imsi-001010000000001-3#1"

	left := ue.EnsurePDN(6)
	left.PDUSessionID = 2

	m.SessionDropped(context.Background(), ue.IMSI(), DefaultERABID, moved.SessionRef)

	if err := m.MMContextAck(context.Background(), ue.Supi(), []uint8{moved.PDUSessionID}); err != nil {
		t.Fatalf("MMContextAck: %v", err)
	}

	if ue.PDNCount() != 0 {
		t.Errorf("PDN connections = %d, want 0: the connection 5GS left behind is still anchored in EPS", ue.PDNCount())
	}

	if ue.EMMState() != EMMDeregistered {
		t.Error("the UE stayed attached to a system it has no PDN connection in")
	}
}

// TS 23.401 §5.5.1.2.2 steps 14 and 19: the source eNB is released once, after
// Forward Relocation Complete and with the successful-handover cause. Releasing
// again from the user-plane switch (step 8) draws an S1AP ERROR INDICATION for a
// UE-associated logical connection the eNB has already torn down
// (TS 36.413 §10.6).
func TestSessionDroppedLeavesTheReleaseToACommittingRelocationToFiveGS(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.TransitionTo(EMMRegistered)

	p := testPDN(ue)
	p.SessionRef = "imsi-001010000000001-3#1"

	if _, ok := m.stageRelocationToFiveGS(ue, ue.Conn(), nil); !ok {
		t.Fatal("could not stage a relocation to 5GS")
	}

	sent := len(cc.sent)

	m.SessionDropped(context.Background(), ue.IMSI(), DefaultERABID, p.SessionRef)

	if len(cc.sent) != sent {
		t.Errorf("sent %d S1AP messages, want 0: the relocation owns the source eNB release", len(cc.sent)-sent)
	}

	if ue.EMMState() == EMMDeregistered {
		t.Error("the UE was detached mid-relocation, so Relocation Complete has no context left to release")
	}
}
