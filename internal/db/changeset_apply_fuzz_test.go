// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"testing"
	"time"
)

// applyChangesetFuzzBudget is generous: a well-formed changeset against a
// small database applies in single-digit milliseconds.
const applyChangesetFuzzBudget = 20 * time.Second

// FuzzApplyChangeset drives peer-supplied bytes through the changeset apply
// path, which is where a Raft log entry crosses into cgo.
//
// The bytes in a CmdChangeset entry are opaque: they are handed to
// sqlite3changeset_apply through the driver with no structural validation on
// the Go side, and every node applies every committed entry. The FSM turns an
// apply error into a panic, so the boundary between "malformed input" and
// "crash the cluster" is exactly this function returning an error rather than
// misbehaving. Decoding the 2-byte command header is not the interesting part
// and cannot fail; this is.
//
// The property is narrow on purpose: applyChangeset must either apply the
// changeset or return an error. It must not panic, corrupt the connection, or
// leave the database mid-transaction — the last of which the follow-up write
// below detects.
func FuzzApplyChangeset(f *testing.F) {
	database := newAtomicTestDB(f)

	f.Add(captureSliceChangesetTB(f, database))
	f.Add([]byte{})
	f.Add([]byte{0x00})
	f.Add([]byte("SQLite format 3\x00"))
	f.Add([]byte{0x54, 0xff, 0xff, 0xff, 0xff})

	f.Fuzz(func(t *testing.T, changeset []byte) {
		fuzzDB := newAtomicTestDB(t)
		ctx := context.Background()

		// sqlite3changeset_apply runs in cgo, where ctx cancellation has no
		// effect: a call that does not return cannot be abandoned. Bound it
		// here so a hang is reported as a failure instead of stalling the
		// whole run — hanging is the more dangerous outcome, since the FSM
		// would block forever without panicking or restarting.
		done := make(chan struct{})

		go func() {
			defer close(done)

			_, _ = fuzzDB.applyChangeset(ctx, &bytesPayload{
				Value:     changeset,
				Operation: "FuzzApplyChangeset",
			}, 1)
		}()

		select {
		case <-done:
		case <-time.After(applyChangesetFuzzBudget):
			t.Fatalf("applyChangeset did not return within %s for %d bytes; "+
				"a changeset apply that never returns blocks the FSM without panicking, "+
				"so the node stops applying entries and never restarts",
				applyChangesetFuzzBudget, len(changeset))
		}

		// The connection must still be usable and outside a transaction. A
		// changeset apply that failed without rolling back would leave
		// BEGIN IMMEDIATE held and this would block or error.
		if _, err := fuzzDB.conn().PlainDB().ExecContext(ctx,
			"UPDATE fsm_state SET lastApplied = lastApplied WHERE id = 1"); err != nil {
			t.Fatalf("database unusable after applying %d fuzz bytes: %v", len(changeset), err)
		}
	})
}

// captureSliceChangesetTB is captureSliceChangeset for a testing.TB, so the
// fuzz seed corpus can reuse it.
func captureSliceChangesetTB(tb testing.TB, database *Database) []byte {
	tb.Helper()

	slice := &NetworkSlice{
		ID:   "01900000-0000-7000-8000-00000000bbbb",
		Sst:  1,
		Name: "fuzz-seed",
	}

	bytes, _, err := database.captureChangeset(context.Background(),
		func(ctx context.Context) (any, error) {
			return database.applyCreateNetworkSlice(ctx, slice)
		}, "CreateNetworkSlice")
	if err != nil {
		tb.Fatalf("capture changeset: %v", err)
	}

	return bytes
}
