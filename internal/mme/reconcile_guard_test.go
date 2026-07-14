// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
	"time"
)

// modifyingAdditionalPDN returns a UE whose default PDN matches its policy and whose
// additional PDN to "ims" differs from its policy by QCI alone, so a reconcile modifies
// only the additional one.
func modifyingAdditionalPDN(t *testing.T, m *MME) (*UeContext, *PdnConnection) {
	t.Helper()

	ue, _ := connectedBearerUE(t, m)

	qos, err := ResolveQoSByAPN(context.Background(), m, ue.imsiOrEmpty(), "internet")
	if err != nil {
		t.Fatal(err)
	}

	testPDN(ue).DnConfig = qos.DnFingerprint()

	imsQoS, err := ResolveQoSByAPN(context.Background(), m, ue.imsiOrEmpty(), "ims")
	if err != nil {
		t.Fatal(err)
	}

	p := ue.EnsurePDN(6)
	p.Apn = "ims"
	p.DnConfig = imsQoS.DnFingerprint()
	p.SessAmbrDLBps = BitRateToBps(imsQoS.SessAmbrDLStr)
	p.SessAmbrULBps = BitRateToBps(imsQoS.SessAmbrULStr)
	p.Qci = imsQoS.QCI + 1
	p.Arp = imsQoS.ARP

	return ue, p
}

// waitForPendingModifyCleared reports whether p's in-flight modification bookkeeping is
// cleared before the deadline.
func waitForPendingModifyCleared(ue *UeContext, p *PdnConnection) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ue.mu.Lock()
		done := !p.Modifying && p.PendingQCI == 0
		ue.mu.Unlock()

		if done {
			return true
		}

		time.Sleep(time.Millisecond)
	}

	return false
}

// TestModifyBearerGuardAbortClearsTheModifiedPDN verifies that the modify guard's abort
// finalizer clears the in-flight modification of the PDN connection it was armed for
// (TS 24.301 §6.4.2.5). The guard is per-bearer so a UE with several PDN connections can
// have a procedure outstanding on each; an additional PDN whose Modifying flag survives
// the abort is skipped by the busy gate on every later reconcile sweep.
func TestModifyBearerGuardAbortClearsTheModifiedPDN(t *testing.T) {
	m := newTestMME(t)
	ue, p := modifyingAdditionalPDN(t, m)

	m.SetESMGuardConfigForTest(20*time.Millisecond, 0)
	m.ReconcileUE(context.Background(), ue)

	if !p.Modifying {
		t.Fatal("additional PDN not marked modifying after a QoS change")
	}

	if !waitForPendingModifyCleared(ue, p) {
		t.Fatalf("additional PDN Modifying = %v, PendingQCI = %d after its own modify guard aborted, want false and 0",
			p.Modifying, p.PendingQCI)
	}
}

// TestModifyBearerGuardAbortLeavesOtherPDNsIntact verifies that an additional PDN's
// modify guard abort does not touch another PDN connection's in-flight modification.
// Clearing the default bearer's bookkeeping from here would strand it: its own guard
// keeps retransmitting, and CommitBearerModification drops the UE's eventual accept as
// out of state, so the configuration the UE accepted never commits.
func TestModifyBearerGuardAbortLeavesOtherPDNsIntact(t *testing.T) {
	m := newTestMME(t)
	ue, p := modifyingAdditionalPDN(t, m)

	// The default bearer has its own modification outstanding, so the reconcile sweep
	// skips it as busy and only the additional PDN arms a guard.
	def := testPDN(ue)
	def.Modifying = true
	def.PendingQCI = 9

	m.SetESMGuardConfigForTest(20*time.Millisecond, 0)
	m.ReconcileUE(context.Background(), ue)

	if !waitForPendingModifyCleared(ue, p) {
		t.Fatal("additional PDN's modification not cleared by its own guard abort")
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	if !def.Modifying || def.PendingQCI != 9 {
		t.Fatalf("default bearer Modifying = %v, PendingQCI = %d after an additional PDN's guard aborted, want true and 9",
			def.Modifying, def.PendingQCI)
	}
}
