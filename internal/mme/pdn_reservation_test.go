// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
)

// TS 36.413 §8.2.1.4: an E-RAB SETUP REQUEST naming "the value that identifies an
// active E-RAB (established before the E-RAB SETUP REQUEST message was received)"
// is reported as failed with "Multiple E-RAB ID instances". A session moved to 5GS
// carries no S1-AP bearer signalling (TS 23.502 §4.11.2.3 step 10), so the eNB
// still holds the E-RAB and its identity must not be issued again.
func TestTransferredEBIIsWithheldUntilTheS1ContextIsReleased(t *testing.T) {
	m := newTestMME(t)
	m.Session = &fakeSessionManager{}

	ue := m.NewUe(&captureConn{}, 7)
	m.RegisterUEForTest(ue, testSubscriber.IMSI)
	ue.ForceStateForTest(EMMRegistered)

	moved := ue.EnsurePDN(DefaultERABID)
	moved.SessionRef = "moved-to-5gs"

	kept := ue.EnsurePDN(DefaultERABID + 1)
	kept.SessionRef = "still-on-eps"

	qos, err := ResolveQoSByAPN(context.Background(), m, testSubscriber.IMSI, "internet")
	if err != nil {
		t.Fatal(err)
	}

	m.SessionTransferred(context.Background(), testSubscriber.IMSI, moved.Ebi, moved.SessionRef)

	p := m.AddPDN(ue, qos)
	if p == nil {
		t.Fatal("AddPDN returned no connection, want a free EPS bearer identity")
	}

	if p.Ebi == moved.Ebi {
		t.Fatalf("allocated EPS bearer identity = %d, want any identity other than the transferred %d", p.Ebi, moved.Ebi)
	}

	m.DropPDN(ue, p.Ebi)
	m.FreeUeConn(ue)

	reused := m.AddPDN(ue, qos)
	if reused == nil {
		t.Fatal("AddPDN returned no connection after the S1 release")
	}

	if reused.Ebi != moved.Ebi {
		t.Errorf("allocated EPS bearer identity = %d after the S1 release, want the released %d", reused.Ebi, moved.Ebi)
	}
}

// A connection is reachable by the reconciler from the moment it is in the UE's
// map, so it is reserved carrying the policy it is being established with and the
// sweep finds nothing to signal (TS 24.301 §6.4.2, §6.4.4.2).
func TestReservedPDNIsReconciledAgainstItsOwnPolicy(t *testing.T) {
	m := newTestMME(t)
	m.Session = &fakeSessionManager{}

	ue, cc := securedUE(t, m)

	qos, err := ResolveQoSByAPN(context.Background(), m, testSubscriber.IMSI, "internet")
	if err != nil {
		t.Fatal(err)
	}

	p := m.AddPDN(ue, qos)
	if p == nil {
		t.Fatal("AddPDN returned no connection")
	}

	sent := cc.count()

	m.ReconcileUE(context.Background(), ue)

	if cc.count() != sent {
		t.Errorf("reconcile sent %d messages against a connection still being established, want 0", cc.count()-sent)
	}

	ue.mu.Lock()
	deactivating := p.Deactivating
	ue.mu.Unlock()

	if deactivating {
		t.Error("Deactivating = true for a connection still being established, want false")
	}
}
