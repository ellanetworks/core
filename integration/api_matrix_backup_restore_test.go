// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ellanetworks/core/client"
)

// runBackupRestoreMatrix exercises backup and restore as a self-contained
// round-trip: it backs up the current state and restores that same snapshot, so
// the datastore is unchanged on success. Restore overwrites the whole datastore,
// so the runner re-waits for readiness and confirms the shared client's session
// survived before returning.
func runBackupRestoreMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	backupPath := filepath.Join(t.TempDir(), "apimat-backup.tar")

	usersBefore, err := c.ListUsers(ctx, &client.ListParams{Page: 1, PerPage: 100})
	if err != nil {
		t.Fatalf("list users before backup: %v", err)
	}

	if err := c.CreateBackup(ctx, &client.CreateBackupParams{Path: backupPath}); err != nil {
		t.Fatalf("create backup: %v", err)
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		t.Fatalf("stat backup file: %v", err)
	}

	if info.Size() == 0 {
		t.Fatalf("backup file is empty")
	}

	if err := c.RestoreBackup(ctx, &client.RestoreBackupParams{Path: backupPath}); err != nil {
		t.Fatalf("restore backup: %v", err)
	}

	if err := waitForEllaCoreReady(ctx, c); err != nil {
		t.Fatalf("wait for ready after restore: %v", err)
	}

	// The restored snapshot is the state captured moments earlier, so the
	// client's token must still be valid and the user set unchanged.
	usersAfter, err := c.ListUsers(ctx, &client.ListParams{Page: 1, PerPage: 100})
	if err != nil {
		t.Fatalf("list users after restore (session lost?): %v", err)
	}

	if usersAfter.TotalCount != usersBefore.TotalCount {
		t.Fatalf("user count after restore: got %d, want %d", usersAfter.TotalCount, usersBefore.TotalCount)
	}
}
