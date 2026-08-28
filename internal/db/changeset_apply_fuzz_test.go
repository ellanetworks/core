// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"testing"
	"time"
)

const applyChangesetFuzzBudget = 20 * time.Second

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

		if _, err := fuzzDB.conn().PlainDB().ExecContext(ctx,
			"UPDATE fsm_state SET lastApplied = lastApplied WHERE id = 1"); err != nil {
			t.Fatalf("database unusable after applying %d fuzz bytes: %v", len(changeset), err)
		}
	})
}

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
