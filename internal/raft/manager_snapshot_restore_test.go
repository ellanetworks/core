// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"io"
	"testing"

	hraft "github.com/hashicorp/raft"
)

type fakeSnapshotStore struct {
	metas []*hraft.SnapshotMeta
}

func (s *fakeSnapshotStore) Create(hraft.SnapshotVersion, uint64, uint64, hraft.Configuration, uint64, hraft.Transport) (hraft.SnapshotSink, error) {
	return nil, io.ErrUnexpectedEOF
}

func (s *fakeSnapshotStore) List() ([]*hraft.SnapshotMeta, error) {
	return s.metas, nil
}

func (s *fakeSnapshotStore) Open(string) (*hraft.SnapshotMeta, io.ReadCloser, error) {
	return nil, nil, io.ErrUnexpectedEOF
}

func snapshotStoreAt(indexes ...uint64) *fakeSnapshotStore {
	metas := make([]*hraft.SnapshotMeta, 0, len(indexes))
	for _, idx := range indexes {
		metas = append(metas, &hraft.SnapshotMeta{Index: idx, Term: 1})
	}

	return &fakeSnapshotStore{metas: metas}
}

func TestConfigureSnapshotRestoreOnStart(t *testing.T) {
	tests := []struct {
		name        string
		lastApplied uint64
		snapshots   []uint64
		wantSkip    bool
	}{
		{
			name:        "database ahead of newest snapshot is preserved",
			lastApplied: 490,
			snapshots:   []uint64{200, 429},
			wantSkip:    true,
		},
		{
			name:        "database level with newest snapshot is preserved",
			lastApplied: 429,
			snapshots:   []uint64{429},
			wantSkip:    true,
		},
		{
			name:        "database behind newest snapshot is rebuilt",
			lastApplied: 100,
			snapshots:   []uint64{429},
			wantSkip:    false,
		},
		{
			name:        "empty database is rebuilt",
			lastApplied: 0,
			snapshots:   []uint64{429},
			wantSkip:    false,
		},
		{
			name:        "no snapshots leaves nothing to restore",
			lastApplied: 0,
			snapshots:   nil,
			wantSkip:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm := NewFSM(newTestApplier(t), t.TempDir())

			if err := fsm.writeLastApplied(tt.lastApplied); err != nil {
				t.Fatalf("seed lastApplied: %v", err)
			}

			cfg := hraft.DefaultConfig()

			if err := configureSnapshotRestoreOnStart(cfg, fsm, snapshotStoreAt(tt.snapshots...)); err != nil {
				t.Fatalf("configureSnapshotRestoreOnStart: %v", err)
			}

			if cfg.NoSnapshotRestoreOnStart != tt.wantSkip {
				t.Fatalf("NoSnapshotRestoreOnStart = %v, want %v (lastApplied=%d, snapshots=%v)",
					cfg.NoSnapshotRestoreOnStart, tt.wantSkip, tt.lastApplied, tt.snapshots)
			}
		})
	}
}

func TestFSM_RestoreOnStartSkippedKeepsRowsAheadOfSnapshot(t *testing.T) {
	applier := newTestApplier(t)
	fsm := NewFSM(applier, t.TempDir())

	if err := fsm.writeLastApplied(429); err != nil {
		t.Fatalf("seed snapshot-era lastApplied: %v", err)
	}

	payload := persistSnapshot(t, fsm)

	if _, err := applier.db.ExecContext(context.Background(), `INSERT INTO t(id, v) VALUES (1, 'applied-after-snapshot')`); err != nil {
		t.Fatalf("insert post-snapshot row: %v", err)
	}

	if err := fsm.writeLastApplied(490); err != nil {
		t.Fatalf("advance lastApplied past snapshot: %v", err)
	}

	cfg := hraft.DefaultConfig()

	if err := configureSnapshotRestoreOnStart(cfg, fsm, snapshotStoreAt(429)); err != nil {
		t.Fatalf("configureSnapshotRestoreOnStart: %v", err)
	}

	if !cfg.NoSnapshotRestoreOnStart {
		if err := fsm.Restore(newReadCloser(payload)); err != nil {
			t.Fatalf("restore: %v", err)
		}
	}

	var count int
	if err := applier.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM t WHERE v = 'applied-after-snapshot'`).Scan(&count); err != nil {
		t.Fatalf("count post-snapshot row: %v", err)
	}

	if count != 1 {
		t.Fatal("post-snapshot row was rolled back; the database was rewritten from a snapshot it was already ahead of")
	}

	got, err := fsm.readLastApplied()
	if err != nil {
		t.Fatalf("read lastApplied: %v", err)
	}

	if got != 490 {
		t.Fatalf("lastApplied = %d, want 490", got)
	}
}
