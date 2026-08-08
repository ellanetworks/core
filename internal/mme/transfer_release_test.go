// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
)

// A PDN connection opened with request type "handover" names a session the UE is
// still running on 5GS until the move commits. Releasing it — which is what an
// aborted attach does, on T3450 expiry or INITIAL CONTEXT SETUP FAILURE — would
// destroy that session and free the UE's address while NR still serves it.
func TestAbortedHandoverAttachDoesNotReleaseThe5GSSession(t *testing.T) {
	m := newTestMME(t)
	sm := m.Session.(*fakeSessionManager)

	ue, _ := securedUE(t, m)
	p := testPDN(ue)
	p.SessionRef = "imsi-001010000000001-3#1"
	p.PendingTransfer = true

	m.ReleaseAllSessions(context.Background(), ue)

	if sm.released {
		t.Error("the anchored session was released though the move never committed: the UE is still running it on 5GS")
	}

	if !sm.abandoned {
		t.Error("the pending move was not abandoned, so the session can never move again")
	}
}

// Once the move has committed the session belongs to EPS, and an ordinary
// release must tear it down as usual.
func TestCommittedTransferReleasesNormally(t *testing.T) {
	m := newTestMME(t)
	sm := m.Session.(*fakeSessionManager)

	ue, _ := securedUE(t, m)
	p := testPDN(ue)
	p.SessionRef = "imsi-001010000000001-3#1"
	p.PendingTransfer = false

	m.ReleaseAllSessions(context.Background(), ue)

	if !sm.released {
		t.Error("a committed EPS session was not released")
	}

	if sm.abandoned {
		t.Error("a committed EPS session was abandoned rather than released")
	}
}

// The flag has to be set from the attach that moved the session and cleared when
// the eNB endpoint reaches the anchor, or the two tests above guard nothing.
func TestPendingTransferLifecycle(t *testing.T) {
	m := newTestMME(t)

	ue, _ := securedUE(t, m)

	qos := &EpsQoS{APN: "internet", QCI: 9}
	bearer := models.EPSBearer{Ref: "imsi-001010000000001-3#1", PDNType: eps.PDNTypeIPv4}

	m.InstallDefaultBearer(ue, qos, bearer, true)

	p := m.DefaultPDN(ue)
	if p == nil {
		t.Fatal("no default PDN after install")
	}

	if !p.PendingTransfer {
		t.Fatal("a connection installed from a handover request is not marked pending")
	}

	m.CommitTransfer(ue, p)

	if p.PendingTransfer {
		t.Error("the pending mark survived the commit, so the session can never be released")
	}
}
