// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestApplyLocalTx_RollsBackPartialApply(t *testing.T) {
	database, err := NewDatabaseWithoutRaft(context.Background(),
		filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	sentinel := errors.New("apply failed after first statement")

	_, err = database.applyLocalTx(context.Background(), "TestOp", func(ctx context.Context) (any, error) {
		for i, name := range []string{"tx-rollback-first", "tx-rollback-second"} {
			if _, err := database.applyCreateNetworkSlice(ctx, &NetworkSlice{
				ID: "01900000-0000-7000-8000-00000000aa0" + string(rune('1'+i)), Sst: 1, Name: name,
			}); err != nil {
				t.Fatalf("applyCreateNetworkSlice %s: %v", name, err)
			}
		}

		return nil, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("applyLocalTx error: want %v, got %v", sentinel, err)
	}

	var n int
	if err := database.PlainDB().QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM network_slices WHERE name LIKE 'tx-rollback-%'").Scan(&n); err != nil {
		t.Fatalf("count network_slices: %v", err)
	}

	if n != 0 {
		t.Fatalf("network_slices: want 0 rows after rollback, got %d", n)
	}
}
