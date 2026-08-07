// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"
)

// The anchor has already torn the session down, so the MME drops the PDN
// connection naming it without releasing it a second time.
func TestSessionReleasedDropsPDNWithoutReleasingTheAnchor(t *testing.T) {
	m := newTestMME(t)
	sm := &fakeSessionManager{}
	m.Session = sm

	ue := m.NewUe(&captureConn{}, 7)
	m.RegisterUEForTest(ue, testSubscriber.IMSI)
	ue.ForceStateForTest(EMMRegistered)

	gone := ue.EnsurePDN(DefaultERABID)
	gone.SessionRef = "released-by-the-anchor"

	kept := ue.EnsurePDN(DefaultERABID + 1)
	kept.SessionRef = "still-on-eps"

	m.SessionReleased(context.Background(), testSubscriber.IMSI, gone.Ebi, gone.SessionRef)

	if sm.released {
		t.Error("released the anchor session, want no release")
	}

	if m.LookupPDN(ue, gone.Ebi) != nil {
		t.Error("PDN connection for the released session is still present")
	}

	if m.LookupPDN(ue, kept.Ebi) == nil {
		t.Error("an unrelated PDN connection was dropped")
	}
}

// A UE that re-established the PDN connection holds a fresh one under the same
// EPS bearer identity, which the late notification for the old session must not
// drop.
func TestSessionReleasedIgnoresAStaleSessionRef(t *testing.T) {
	m := newTestMME(t)
	m.Session = &fakeSessionManager{}

	ue := m.NewUe(&captureConn{}, 7)
	m.RegisterUEForTest(ue, testSubscriber.IMSI)
	ue.ForceStateForTest(EMMRegistered)

	current := ue.EnsurePDN(DefaultERABID)
	current.SessionRef = "re-established"

	m.SessionReleased(context.Background(), testSubscriber.IMSI, current.Ebi, "released-by-the-anchor")

	if m.LookupPDN(ue, current.Ebi) != current {
		t.Error("the notification for the earlier session dropped the current PDN connection")
	}
}

// An attached EPS UE always holds at least one PDN connection (TS 23.401
// §5.10.3), so losing its last one detaches it.
func TestSessionReleasedDetachesAUEThatLosesItsLastPDN(t *testing.T) {
	m := newTestMME(t)
	m.Session = &fakeSessionManager{}

	ue := m.NewUe(&captureConn{}, 7)
	m.RegisterUEForTest(ue, testSubscriber.IMSI)
	ue.ForceStateForTest(EMMRegistered)

	only := ue.EnsurePDN(DefaultERABID)
	only.SessionRef = "released-by-the-anchor"

	m.SessionReleased(context.Background(), testSubscriber.IMSI, only.Ebi, only.SessionRef)

	if got := ue.EMMState(); got != EMMDeregistered {
		t.Errorf("EMM state = %v, want %v", got, EMMDeregistered)
	}
}

// TS 23.502 §4.11.2.3 step 10 releases the EPC resources of a transferred PDN
// connection "except the steps 4-7" of TS 23.401 §5.4.4.1, and step 4a is the
// explicit detach. A dual-registration UE that moves its last PDN connection to
// 5GS stays EMM-REGISTERED.
func TestSessionTransferredKeepsAUEThatMovesItsLastPDN(t *testing.T) {
	m := newTestMME(t)
	m.Session = &fakeSessionManager{}

	ue := m.NewUe(&captureConn{}, 7)
	m.RegisterUEForTest(ue, testSubscriber.IMSI)
	ue.ForceStateForTest(EMMRegistered)

	only := ue.EnsurePDN(DefaultERABID)
	only.SessionRef = "moved-to-5gs"

	m.SessionTransferred(context.Background(), testSubscriber.IMSI, only.Ebi, only.SessionRef)

	if m.LookupPDN(ue, only.Ebi) != nil {
		t.Error("PDN connection for the moved session is still present")
	}

	if got := ue.EMMState(); got != EMMRegistered {
		t.Errorf("EMM state = %v, want %v", got, EMMRegistered)
	}
}
