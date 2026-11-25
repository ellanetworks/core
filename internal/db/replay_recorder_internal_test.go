// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

type recordedCall struct {
	entries        []ellaraft.Entry
	results        []string
	err            string
	maxSubscribers int
}

type stateCheckpoint struct {
	applied  int
	tables   map[string]string
	external bool
}

type replayRecording struct {
	mu          sync.Mutex
	db          *Database
	test        string
	dir         string
	snapshot    string
	monitor     *sql.DB
	dataVersion int64
	calls       []recordedCall
	entries     int
	checkpoints []stateCheckpoint
	skip        string
	closed      bool
}

type replayRecorder struct {
	mu         sync.Mutex
	dir        string
	recordings map[*Database]*replayRecording
	order      []*replayRecording
}

func newReplayRecorder() (*replayRecorder, error) {
	dir, err := os.MkdirTemp("", "ella-replay-")
	if err != nil {
		return nil, err
	}

	return &replayRecorder{dir: dir, recordings: map[*Database]*replayRecording{}}, nil
}

func (r *replayRecorder) install() {
	observeApply = r.observeApply
	observeClose = r.observeClose
}

func (r *replayRecorder) uninstall() {
	observeApply = nil
	observeClose = nil
}

func (r *replayRecorder) cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, rec := range r.order {
		if rec.monitor != nil {
			_ = rec.monitor.Close()
		}
	}

	_ = os.RemoveAll(r.dir)
}

func (r *replayRecorder) recordingFor(db *Database) *replayRecording {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.recordings[db]
}

func (r *replayRecorder) all() []*replayRecording {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.order)
}

func testNameFromPath(path string) string {
	base := filepath.Base(filepath.Dir(filepath.Dir(path)))

	return strings.TrimRight(base, "0123456789")
}

func (r *replayRecorder) observeApply(db *Database, entries []ellaraft.Entry) func([]any, error) {
	r.mu.Lock()

	rec, ok := r.recordings[db]
	if !ok {
		rec = &replayRecording{db: db, test: testNameFromPath(db.dbPath), dir: filepath.Join(r.dir, fmt.Sprintf("%04d", len(r.order)))}
		r.recordings[db] = rec
		r.order = append(r.order, rec)
	}

	r.mu.Unlock()

	rec.mu.Lock()

	if rec.skip == "" && db.raftManager != nil {
		rec.skip = "raft-backed database"
	}

	if rec.skip == "" && rec.snapshot == "" {
		if err := rec.start(); err != nil {
			rec.skip = "snapshot failed: " + err.Error()
		}
	}

	if rec.skip == "" {
		rec.checkExternal(rec.entries)
	}

	maxSubscribers := db.maxSubscribers

	return func(out []any, err error) {
		defer rec.mu.Unlock()

		if rec.skip != "" {
			return
		}

		call := recordedCall{entries: slices.Clone(entries), results: encodeApplyResults(out), maxSubscribers: maxSubscribers}
		if err != nil {
			call.err = err.Error()
		}

		rec.calls = append(rec.calls, call)
		rec.entries += len(entries)

		dv, dvErr := rec.readDataVersion()
		if dvErr != nil {
			rec.skip = "data_version failed: " + dvErr.Error()
			return
		}

		rec.dataVersion = dv
	}
}

func (r *replayRecorder) observeClose(db *Database) {
	rec := r.recordingFor(db)
	if rec == nil {
		return
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	rec.finish()
}

func (r *replayRecorder) finishAll() {
	for _, rec := range r.all() {
		rec.mu.Lock()
		rec.finish()
		rec.mu.Unlock()
	}
}

func (rec *replayRecording) start() error {
	if err := os.MkdirAll(rec.dir, 0o700); err != nil {
		return err
	}

	rec.snapshot = filepath.Join(rec.dir, "snapshot.db")

	if _, err := rec.db.writer().ExecContext(context.Background(), "VACUUM INTO ?", rec.snapshot); err != nil {
		return err
	}

	monitor, err := sql.Open("sqlite3", rec.db.dbPath+"?_busy_timeout=5000")
	if err != nil {
		return err
	}

	monitor.SetMaxOpenConns(1)
	monitor.SetMaxIdleConns(1)
	monitor.SetConnMaxLifetime(0)
	monitor.SetConnMaxIdleTime(0)
	rec.monitor = monitor

	dv, err := rec.readDataVersion()
	if err != nil {
		return err
	}

	rec.dataVersion = dv

	return nil
}

func (rec *replayRecording) readDataVersion() (int64, error) {
	var dv int64

	err := rec.monitor.QueryRowContext(context.Background(), "PRAGMA data_version").Scan(&dv)

	return dv, err
}

func (rec *replayRecording) checkExternal(applied int) {
	dv, err := rec.readDataVersion()
	if err != nil {
		rec.skip = "data_version failed: " + err.Error()
		return
	}

	if dv == rec.dataVersion {
		return
	}

	rec.dataVersion = dv
	rec.checkpoint(applied, true)
}

func (rec *replayRecording) checkpoint(applied int, external bool) {
	tables, err := replicatedTableHashes(context.Background(), rec.monitor)
	if err != nil {
		rec.skip = "state hash failed: " + err.Error()
		return
	}

	if n := len(rec.checkpoints); n > 0 && rec.checkpoints[n-1].applied == applied {
		rec.checkpoints[n-1] = stateCheckpoint{applied: applied, tables: tables, external: external || rec.checkpoints[n-1].external}
		return
	}

	rec.checkpoints = append(rec.checkpoints, stateCheckpoint{applied: applied, tables: tables, external: external})
}

func (rec *replayRecording) finish() {
	if rec.closed || rec.skip != "" || rec.monitor == nil {
		rec.closed = true
		return
	}

	rec.closed = true

	dv, err := rec.readDataVersion()
	if err != nil {
		rec.skip = "final data_version failed: " + err.Error()
		return
	}

	rec.checkpoint(rec.entries, dv != rec.dataVersion)

	_ = rec.monitor.Close()
	rec.monitor = nil
}

func encodeApplyResults(out []any) []string {
	encoded := make([]string, len(out))

	for i, o := range out {
		results, ok := o.([]commandResult)
		if !ok {
			encoded[i] = fmt.Sprintf("entry:%T", o)
			continue
		}

		parts := make([]string, len(results))

		for j, res := range results {
			if res.err != nil {
				parts[j] = "error: " + res.err.Error()
				continue
			}

			value, err := json.Marshal(res.value)
			if err != nil {
				parts[j] = fmt.Sprintf("unencodable %T: %v", res.value, err)
				continue
			}

			parts[j] = string(value)
		}

		encoded[i] = strings.Join(parts, "\n")
	}

	return encoded
}

func replicatedTableHashes(ctx context.Context, q rowQuerier) (map[string]string, error) {
	out := make(map[string]string, len(replicatedTables))

	for _, table := range replicatedTables {
		h := sha256.New()

		if err := hashTable(ctx, q, h, table); err != nil {
			return nil, fmt.Errorf("hash %s: %w", table, err)
		}

		out[table] = hex.EncodeToString(h.Sum(nil))
	}

	return out, nil
}

func (rec *replayRecording) operations(from, to int) []string {
	var ops []string

	pos := 0

	for _, call := range rec.calls {
		for _, e := range call.entries {
			if pos >= from && pos < to {
				var b commandBatch
				if err := json.Unmarshal(e.Data, &b); err == nil {
					for _, c := range b.Commands {
						ops = append(ops, fmt.Sprintf("%d:%s", e.Index, c.Op))
					}
				}
			}

			pos++
		}
	}

	return ops
}

func (rec *replayRecording) unregisteredOperations() []string {
	seen := map[string]bool{}

	var out []string

	for _, call := range rec.calls {
		for _, e := range call.entries {
			var b commandBatch
			if err := json.Unmarshal(e.Data, &b); err != nil {
				continue
			}

			for _, c := range b.Commands {
				if _, ok := commandHandlerFor(c.Op); ok || seen[c.Op] {
					continue
				}

				if _, retired := retiredChangesetOps[c.Op]; retired {
					continue
				}

				seen[c.Op] = true
				out = append(out, c.Op)
			}
		}
	}

	return out
}
