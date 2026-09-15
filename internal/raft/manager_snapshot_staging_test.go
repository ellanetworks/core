// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanSnapshotStaging(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	raftDir := filepath.Join(dataDir, "raft")
	legacyDir := filepath.Join(raftDir, "snapshots", "tmp")
	stagingDir := snapshotStagingDir(dataDir)

	keepDir := filepath.Join(raftDir, "snapshots", "2-8-1700000000000")

	for _, dir := range []string{legacyDir, stagingDir, keepDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("create %s: %v", dir, err)
		}

		if err := os.WriteFile(filepath.Join(dir, "orphan.db"), []byte("x"), 0o600); err != nil {
			t.Fatalf("write orphan in %s: %v", dir, err)
		}
	}

	cleanSnapshotStaging(dataDir, raftDir)

	for _, dir := range []string{legacyDir, stagingDir} {
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("staging dir %s must be removed; stat err=%v", dir, err)
		}
	}

	if _, err := os.Stat(filepath.Join(keepDir, "orphan.db")); err != nil {
		t.Errorf("real snapshot must survive the sweep: %v", err)
	}
}

func TestCleanSnapshotStagingNoDirs(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()

	cleanSnapshotStaging(dataDir, filepath.Join(dataDir, "raft"))
}
