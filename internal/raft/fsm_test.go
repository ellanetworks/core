// Copyright 2026 Ella Networks

package raft

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	hraft "github.com/hashicorp/raft"
	_ "github.com/mattn/go-sqlite3"
)

// testApplier is a minimal Applier backed by a real SQLite file. Apply records
// commands in the order they arrive so tests can assert on replay behaviour.
type testApplier struct {
	mu         sync.Mutex
	dbPath     string
	db         *sql.DB
	commands   []*Command
	applyErr   error
	reopenHook func()
}

func newTestApplier(t *testing.T) *testApplier {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "shared.db")

	a := &testApplier{dbPath: path}

	if err := a.open(); err != nil {
		t.Fatalf("open initial db: %v", err)
	}

	if _, err := a.db.ExecContext(context.Background(), `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	return a
}

func (a *testApplier) open() error {
	db, err := sql.Open("sqlite3", a.dbPath)
	if err != nil {
		return err
	}

	db.SetMaxOpenConns(1)
	a.db = db

	return nil
}

func (a *testApplier) ApplyCommand(_ context.Context, cmd *Command) (any, error) {
	a.mu.Lock()
	a.commands = append(a.commands, cmd)
	a.mu.Unlock()

	return nil, a.applyErr
}

func (a *testApplier) SharedPlainDB() *sql.DB { return a.db }
func (a *testApplier) SharedPath() string     { return a.dbPath }

func (a *testApplier) ReopenShared(_ context.Context) error {
	if a.db != nil {
		_ = a.db.Close()
	}

	if err := a.open(); err != nil {
		return err
	}

	if a.reopenHook != nil {
		a.reopenHook()
	}

	return nil
}

func (a *testApplier) seen() []*Command {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]*Command, len(a.commands))
	copy(out, a.commands)

	return out
}

// TestFSM_Apply_AdvancesAppliedIndex confirms that AppliedIndex tracks the
// highest index successfully applied, and that applier errors still advance
// the index per hashicorp/raft semantics (the error is returned as the
// response but the log is committed).
func TestFSM_Apply_AdvancesAppliedIndex(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	cmd, err := NewCommand(CmdCreateSubscriber, map[string]string{"imsi": "001"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp := fsm.Apply(&hraft.Log{Index: 7, Data: data})
	if resp != nil {
		t.Fatalf("expected nil response, got %v", resp)
	}

	if got := fsm.AppliedIndex(); got != 7 {
		t.Fatalf("applied index: want 7, got %d", got)
	}

	if len(a.seen()) != 1 {
		t.Fatalf("expected 1 command applied, got %d", len(a.seen()))
	}
}

// TestFSM_Apply_BadPayload returns an error and leaves appliedIndex unchanged.
func TestFSM_Apply_BadPayload(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	// One-byte payload is shorter than the 2-byte header.
	resp := fsm.Apply(&hraft.Log{Index: 3, Data: []byte{0x01}})

	err, ok := resp.(error)
	if !ok {
		t.Fatalf("expected error response, got %T: %v", resp, resp)
	}

	if err == nil {
		t.Fatal("expected non-nil error")
	}

	if got := fsm.AppliedIndex(); got != 0 {
		t.Fatalf("applied index must not advance on unmarshal failure, got %d", got)
	}
}

// TestFSM_Apply_PropagatesApplierError surfaces the applier's error as the
// Apply response but still advances the applied index.
func TestFSM_Apply_PropagatesApplierError(t *testing.T) {
	a := newTestApplier(t)
	a.applyErr = errors.New("boom")

	fsm := NewFSM(a, t.TempDir())

	cmd, err := NewCommand(CmdDeleteSubscriber, map[string]string{"value": "x"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	resp := fsm.Apply(&hraft.Log{Index: 5, Data: data})

	gotErr, ok := resp.(error)
	if !ok {
		t.Fatalf("expected error response, got %T: %v", resp, resp)
	}

	if gotErr.Error() != "boom" {
		t.Fatalf("want boom, got %v", gotErr)
	}

	if got := fsm.AppliedIndex(); got != 0 {
		t.Fatalf("applied index must not advance when applier fails, got %d", got)
	}
}

// TestFSM_SnapshotRestoreRoundTrip writes rows to the source DB, takes a
// Snapshot, Persists it to a buffer, then Restores into a different applier
// pointing at a fresh DB file and verifies the rows arrive.
func TestFSM_SnapshotRestoreRoundTrip(t *testing.T) {
	src := newTestApplier(t)

	for i := 1; i <= 3; i++ {
		if _, err := src.db.ExecContext(context.Background(), `INSERT INTO t(id, v) VALUES (?, ?)`, i, "row"); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	srcFSM := NewFSM(src, t.TempDir())

	snap, err := srcFSM.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	sink := &memSink{}

	if err := snap.Persist(sink); err != nil {
		t.Fatalf("persist: %v", err)
	}

	snap.Release()

	if sink.buf.Len() == 0 {
		t.Fatal("snapshot bytes are empty")
	}

	// Destination applier starts with its own empty schema. Restore must
	// replace it with the source contents.
	dst := newTestApplier(t)
	dstFSM := NewFSM(dst, t.TempDir())

	rc := newReadCloser(sink.buf.Bytes())
	if err := dstFSM.Restore(rc); err != nil {
		t.Fatalf("restore: %v", err)
	}

	var count int
	if err := dst.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM t`).Scan(&count); err != nil {
		t.Fatalf("count after restore: %v", err)
	}

	if count != 3 {
		t.Fatalf("want 3 rows after restore, got %d", count)
	}
}

// TestFSM_Snapshot_ProducesValidSQLite verifies the snapshot bytes can be
// opened as a SQLite database independently of the applier round-trip.
func TestFSM_Snapshot_ProducesValidSQLite(t *testing.T) {
	a := newTestApplier(t)

	if _, err := a.db.ExecContext(context.Background(), `INSERT INTO t(id, v) VALUES (1, 'hello')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	fsm := NewFSM(a, t.TempDir())

	snap, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}

	sink := &memSink{}
	if err := snap.Persist(sink); err != nil {
		t.Fatalf("persist: %v", err)
	}

	snap.Release()

	tmp := filepath.Join(t.TempDir(), "out.db")

	if err := os.WriteFile(tmp, sink.buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write snapshot: %v", err)
	}

	conn, err := sql.Open("sqlite3", tmp)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}

	defer func() { _ = conn.Close() }()

	var v string
	if err := conn.QueryRowContext(context.Background(), `SELECT v FROM t WHERE id = 1`).Scan(&v); err != nil {
		t.Fatalf("query snapshot: %v", err)
	}

	if v != "hello" {
		t.Fatalf("want hello, got %q", v)
	}
}

// TestCommand_RoundTrip covers MarshalBinary/UnmarshalCommand for a typical
// payload.
func TestCommand_RoundTrip(t *testing.T) {
	type payload struct {
		IMSI string `json:"imsi"`
	}

	cmd, err := NewCommand(CmdCreateSubscriber, payload{IMSI: "001010000000001"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got, err := UnmarshalCommand(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != CmdCreateSubscriber {
		t.Fatalf("type: want %v, got %v", CmdCreateSubscriber, got.Type)
	}

	var p payload
	if err := json.Unmarshal(got.Payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if p.IMSI != "001010000000001" {
		t.Fatalf("imsi: want 001010000000001, got %q", p.IMSI)
	}
}

// memSink is an in-memory hraft.SnapshotSink used for tests.
type memSink struct {
	buf       bytes.Buffer
	closed    bool
	cancelled bool
}

func (s *memSink) Write(p []byte) (int, error) { return s.buf.Write(p) }
func (s *memSink) Close() error                { s.closed = true; return nil }
func (s *memSink) ID() string                  { return "test-sink" }
func (s *memSink) Cancel() error               { s.cancelled = true; return nil }

type byteReadCloser struct {
	*bytes.Reader
}

func (byteReadCloser) Close() error { return nil }

func newReadCloser(b []byte) byteReadCloser {
	return byteReadCloser{Reader: bytes.NewReader(b)}
}
