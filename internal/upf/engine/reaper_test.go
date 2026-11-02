// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

type reaperWaiter struct{ err error }

func (w *reaperWaiter) Wait() error { return w.err }

func addPendingSession(conn *SessionEngine, seid uint64, deleted bool) *Session {
	session := NewSession(seid)
	session.qerMeters = map[uint32]struct{}{1: {}}
	session.deleted = deleted
	conn.AddSession(seid, session)

	return session
}

func TestReapDeletedSessionsRetriesUntilCleanupSucceeds(t *testing.T) {
	conn := newTestEngine()
	barrier := &reaperWaiter{err: errors.New("barrier failed")}
	conn.programs = barrier

	pending := addPendingSession(conn, 1, false)
	live := addPendingSession(conn, 2, false)

	if _, err := conn.DeleteSession(t.Context(), &models.DeleteRequest{SEID: 1}); err == nil {
		t.Fatal("delete succeeded although the barrier failed")
	}

	if got := conn.pendingCleanupSEIDs(); !slices.Equal(got, []uint64{1}) {
		t.Fatalf("pending cleanup = %v, want [1]", got)
	}

	if _, err := conn.ModifySession(t.Context(), &models.ModifyRequest{SEID: 1}); !errors.Is(err, models.ErrSessionUnrecoverable) {
		t.Fatalf("modify of a session awaiting cleanup = %v, want ErrSessionUnrecoverable", err)
	}

	if cleaned, left := conn.ReapDeletedSessions(t.Context()); len(cleaned) != 0 || left != 1 {
		t.Fatalf("reap with a failing barrier = (%v, %d), want ([], 1)", cleaned, left)
	}

	barrier.err = nil

	if cleaned, left := conn.ReapDeletedSessions(t.Context()); !slices.Equal(cleaned, []uint64{1}) || left != 0 {
		t.Fatalf("reap = (%v, %d), want ([1], 0)", cleaned, left)
	}

	if conn.GetSession(1) != nil || len(pending.qerMeters) != 0 {
		t.Fatal("the reaped session was not fully removed")
	}

	if conn.GetSession(2) != live || live.deleted || len(live.qerMeters) != 1 {
		t.Fatal("the reaper touched a live session")
	}

	if cleaned, left := conn.ReapDeletedSessions(t.Context()); len(cleaned) != 0 || left != 0 {
		t.Fatalf("reap with nothing pending = (%v, %d)", cleaned, left)
	}
}

func TestReapDeletedSessionsSkipsABusySession(t *testing.T) {
	conn := newTestEngine()
	session := addPendingSession(conn, 1, true)

	session.opMu.Lock()

	if got := conn.pendingCleanupSEIDs(); len(got) != 0 {
		t.Fatalf("pending cleanup = %v; a session whose operation is running must be skipped", got)
	}

	session.opMu.Unlock()

	if got := conn.pendingCleanupSEIDs(); !slices.Equal(got, []uint64{1}) {
		t.Fatalf("pending cleanup = %v, want [1]", got)
	}
}

func TestAddCleanupPDRsQueuesEachEntryOnce(t *testing.T) {
	session := NewSession(1)

	ue := netip.MustParseAddr("10.0.0.1")
	keyed := SPDRInfo{PdrID: 2, UEIP: ue}
	tunnel := SPDRInfo{PdrID: 1, TeID: 7}
	moved := SPDRInfo{PdrID: 1, TeID: 8}

	session.addCleanupPDRs(keyed, tunnel)
	session.addCleanupPDRs(tunnel, keyed, moved)

	if got := len(session.cleanupPDRs); got != 3 {
		t.Fatalf("queued %d PDRs (%+v), want 3: each entry once, both keys of the moved PDR", got, session.cleanupPDRs)
	}
}
