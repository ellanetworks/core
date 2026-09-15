// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/raft"
)

func TestCleanSnapshotStaging(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	raftDir := filepath.Join(dataDir, "raft")
	legacyDir := filepath.Join(raftDir, "snapshots", "tmp")
	stagingDir := snapshotStagingDir(dataDir)

	for _, dir := range []string{legacyDir, stagingDir} {
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
}

func TestCleanSnapshotStagingRemovesIncompleteRaftSnapshot(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	raftDir := filepath.Join(dataDir, "raft")

	store, err := raft.NewFileSnapshotStore(raftDir, 3, os.Stderr)
	if err != nil {
		t.Fatalf("new snapshot store: %v", err)
	}

	done, err := store.Create(1, 5, 1, raft.Configuration{}, 0, nil)
	if err != nil {
		t.Fatalf("create complete snapshot: %v", err)
	}

	if _, err := done.Write([]byte("complete")); err != nil {
		t.Fatalf("write complete snapshot: %v", err)
	}

	if err := done.Close(); err != nil {
		t.Fatalf("close complete snapshot: %v", err)
	}

	orphan, err := store.Create(1, 10, 2, raft.Configuration{}, 0, nil)
	if err != nil {
		t.Fatalf("create incomplete snapshot: %v", err)
	}

	if _, err := orphan.Write([]byte("partial")); err != nil {
		t.Fatalf("write incomplete snapshot: %v", err)
	}

	cleanSnapshotStaging(dataDir, raftDir)

	entries, err := os.ReadDir(filepath.Join(raftDir, "snapshots"))
	if err != nil {
		t.Fatalf("read snapshot dir: %v", err)
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == incompleteSnapshotSuffix {
			t.Errorf("incomplete snapshot %s must be removed", entry.Name())
		}
	}

	reopened, err := raft.NewFileSnapshotStore(raftDir, 3, os.Stderr)
	if err != nil {
		t.Fatalf("reopen snapshot store: %v", err)
	}

	snaps, err := reopened.List()
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}

	if len(snaps) != 1 {
		t.Fatalf("completed snapshot must survive the sweep; got %d snapshots", len(snaps))
	}

	if snaps[0].Index != 5 {
		t.Errorf("surviving snapshot index = %d, want 5", snaps[0].Index)
	}
}
