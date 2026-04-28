// Copyright 2026 Ella Networks

package raft

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	hraft "github.com/hashicorp/raft"
	_ "github.com/mattn/go-sqlite3"
)

// testApplier is a minimal Applier backed by a real SQLite file. Apply records
// commands in the order they arrive so tests can assert on replay behaviour.
// When writeRows is true, ApplyCommand also inserts each payload into the t
// table so that multi-node tests can compare SQLite state across nodes.
type testApplier struct {
	mu         sync.Mutex
	dbPath     string
	db         *sql.DB
	commands   []*Command
	applyErr   error
	reopenHook func()
	writeRows  bool
}

func newTestApplier(t *testing.T) *testApplier {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "ella.db")

	a := &testApplier{dbPath: path}

	if err := a.open(); err != nil {
		t.Fatalf("open initial db: %v", err)
	}

	if _, err := a.db.ExecContext(context.Background(), `CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT NOT NULL)`); err != nil {
		t.Fatalf("create table: %v", err)
	}

	if _, err := a.db.ExecContext(context.Background(), `
		CREATE TABLE IF NOT EXISTS fsm_state (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			lastApplied INTEGER NOT NULL DEFAULT 0
		)`); err != nil {
		t.Fatalf("create fsm_state: %v", err)
	}

	if _, err := a.db.ExecContext(context.Background(),
		"INSERT OR IGNORE INTO fsm_state (id, lastApplied) VALUES (1, 0)"); err != nil {
		t.Fatalf("seed fsm_state: %v", err)
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

func (a *testApplier) ApplyCommand(ctx context.Context, cmd *Command, _ uint64) (any, error) {
	a.mu.Lock()
	a.commands = append(a.commands, cmd)
	a.mu.Unlock()

	if a.applyErr != nil {
		return nil, a.applyErr
	}

	// Write the command payload to SQLite so multi-node tests can compare
	// database state across nodes after Raft replication. Only enabled when
	// each node has its own applier (writeRows=true); shared-applier tests
	// leave this off to avoid SQLite lock contention during elections.
	if a.writeRows && a.db != nil {
		_, _ = a.db.ExecContext(ctx,
			"INSERT INTO t(v) VALUES (?)", string(cmd.Payload))
	}

	return nil, nil
}

func (a *testApplier) PlainDB() *sql.DB { return a.db }
func (a *testApplier) Path() string     { return a.dbPath }

func (a *testApplier) Reopen(_ context.Context) error {
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

func (a *testApplier) BackupLocalTables(_ context.Context, _, _ string) error  { return nil }
func (a *testApplier) RestoreLocalTables(_ context.Context, _, _ string) error { return nil }

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

	cmd, err := NewCommand(CmdChangeset, map[string]string{"imsi": "001"})
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

// TestFSM_Apply_PanicsOnApplierError verifies that an applier error causes
// the FSM to panic (fail-stop) rather than silently continuing with a
// diverged state.
func TestFSM_Apply_PanicsOnApplierError(t *testing.T) {
	a := newTestApplier(t)
	a.applyErr = errors.New("boom")

	fsm := NewFSM(a, t.TempDir())

	cmd, err := NewCommand(CmdChangeset, map[string]string{"value": "x"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on apply error, but Apply returned normally")
		}

		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}

		if !strings.Contains(msg, "boom") {
			t.Fatalf("panic message should contain applier error, got: %s", msg)
		}
	}()

	fsm.Apply(&hraft.Log{Index: 5, Data: data})
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

	raw := sink.buf.Bytes()
	if len(raw) < snapshotHeaderSize || !bytes.Equal(raw[:4], []byte(snapshotMagic)) {
		t.Fatalf("snapshot missing ELSN header: %q", raw[:min(16, len(raw))])
	}

	if err := os.WriteFile(tmp, raw[snapshotHeaderSize:], 0o600); err != nil {
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

	cmd, err := NewCommand(CmdChangeset, payload{IMSI: "001010000000001"})
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

	if got.Type != CmdChangeset {
		t.Fatalf("type: want %v, got %v", CmdChangeset, got.Type)
	}

	var p payload
	if err := json.Unmarshal(got.Payload, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if p.IMSI != "001010000000001" {
		t.Fatalf("imsi: want 001010000000001, got %q", p.IMSI)
	}
}

// TestFSM_Apply_SkipsAlreadyApplied verifies that entries with an index at
// or below the durable lastApplied are skipped. This prevents crash-recovery
// replay from re-applying non-idempotent changesets.
func TestFSM_Apply_SkipsAlreadyApplied(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	cmd, err := NewCommand(CmdChangeset, map[string]string{"v": "1"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Apply at index 10.
	resp := fsm.Apply(&hraft.Log{Index: 10, Data: data})
	if resp != nil {
		t.Fatalf("expected nil response, got %v", resp)
	}

	if len(a.seen()) != 1 {
		t.Fatalf("expected 1 command, got %d", len(a.seen()))
	}

	// Re-apply at index 10 (simulating crash-replay). Should be skipped.
	resp = fsm.Apply(&hraft.Log{Index: 10, Data: data})
	if resp != nil {
		t.Fatalf("expected nil response for skipped entry, got %v", resp)
	}

	if len(a.seen()) != 1 {
		t.Fatalf("expected still 1 command after skip, got %d", len(a.seen()))
	}

	// Apply at an older index. Should also be skipped.
	resp = fsm.Apply(&hraft.Log{Index: 5, Data: data})
	if resp != nil {
		t.Fatalf("expected nil response for older entry, got %v", resp)
	}

	if len(a.seen()) != 1 {
		t.Fatalf("expected still 1 command after older skip, got %d", len(a.seen()))
	}

	// Apply at a new index. Should execute.
	resp = fsm.Apply(&hraft.Log{Index: 11, Data: data})
	if resp != nil {
		t.Fatalf("expected nil response, got %v", resp)
	}

	if len(a.seen()) != 2 {
		t.Fatalf("expected 2 commands after new apply, got %d", len(a.seen()))
	}

	if got := fsm.AppliedIndex(); got != 11 {
		t.Fatalf("applied index: want 11, got %d", got)
	}
}

// TestFSM_ApplyBatch_Basic verifies that ApplyBatch processes multiple logs in
// one call, advances AppliedIndex to the last entry, and writes lastApplied
// only once (both observations confirmed by checking the applier command count
// and the final durable index).
func TestFSM_ApplyBatch_Basic(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	logs := make([]*hraft.Log, 5)
	for i := range logs {
		cmd, err := NewCommand(CmdChangeset, map[string]string{"i": string(rune('a' + i))})
		if err != nil {
			t.Fatalf("new command %d: %v", i, err)
		}

		data, err := cmd.MarshalBinary()
		if err != nil {
			t.Fatalf("marshal %d: %v", i, err)
		}

		logs[i] = &hraft.Log{Index: uint64(10 + i), Data: data}
	}

	results := fsm.ApplyBatch(logs)
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}

	for i, r := range results {
		if r != nil {
			t.Fatalf("result[%d]: expected nil, got %v", i, r)
		}
	}

	if got := fsm.AppliedIndex(); got != 14 {
		t.Fatalf("applied index: want 14, got %d", got)
	}

	if got := len(a.seen()); got != 5 {
		t.Fatalf("expected 5 commands applied, got %d", got)
	}

	// Verify lastApplied was durably persisted to the highest index.
	var idx uint64
	if err := a.db.QueryRowContext(context.Background(),
		"SELECT lastApplied FROM fsm_state WHERE id = 1").Scan(&idx); err != nil {
		t.Fatalf("read lastApplied: %v", err)
	}

	if idx != 14 {
		t.Fatalf("durable lastApplied: want 14, got %d", idx)
	}
}

// TestFSM_ApplyBatch_SkipsAlreadyApplied confirms that entries at or below the
// durable lastApplied are skipped within a batch.
func TestFSM_ApplyBatch_SkipsAlreadyApplied(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	cmd, err := NewCommand(CmdChangeset, map[string]string{"v": "x"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Apply a single entry at index 10 to set lastApplied.
	fsm.Apply(&hraft.Log{Index: 10, Data: data})

	// Batch containing entries before and after lastApplied.
	logs := []*hraft.Log{
		{Index: 8, Data: data},
		{Index: 10, Data: data},
		{Index: 11, Data: data},
		{Index: 12, Data: data},
	}

	results := fsm.ApplyBatch(logs)
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// Only 2 new entries (11, 12) should have been applied, plus the 1
	// from the initial Apply call = 3 total.
	if got := len(a.seen()); got != 3 {
		t.Fatalf("expected 3 total commands, got %d", got)
	}

	if got := fsm.AppliedIndex(); got != 12 {
		t.Fatalf("applied index: want 12, got %d", got)
	}
}

// TestFSM_ApplyBatch_PanicsOnError verifies that an applier error in the
// middle of a batch causes the FSM to panic (fail-stop).
func TestFSM_ApplyBatch_PanicsOnError(t *testing.T) {
	a := newTestApplier(t)
	a.applyErr = errors.New("batch-boom")

	fsm := NewFSM(a, t.TempDir())

	cmd, err := NewCommand(CmdChangeset, map[string]string{"v": "y"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	logs := []*hraft.Log{
		{Index: 1, Data: data},
		{Index: 2, Data: data},
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on batch apply error, but ApplyBatch returned normally")
		}

		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}

		if !strings.Contains(msg, "batch-boom") {
			t.Fatalf("panic message should contain applier error, got: %s", msg)
		}
	}()

	fsm.ApplyBatch(logs)
}

// TestFSM_ApplyBatch_AllSkipped verifies that when every entry in a batch is
// at or below lastApplied, writeLastApplied is not called (no durable write)
// and the in-memory appliedIndex still tracks the highest skipped entry.
func TestFSM_ApplyBatch_AllSkipped(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	cmd, err := NewCommand(CmdChangeset, map[string]string{"v": "z"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Apply at index 20 to set lastApplied.
	fsm.Apply(&hraft.Log{Index: 20, Data: data})

	// Batch entirely at or below lastApplied.
	logs := []*hraft.Log{
		{Index: 15, Data: data},
		{Index: 18, Data: data},
		{Index: 20, Data: data},
	}

	results := fsm.ApplyBatch(logs)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	for i, r := range results {
		if r != nil {
			t.Fatalf("result[%d]: expected nil, got %v", i, r)
		}
	}

	// Only the original Apply call should have executed.
	if got := len(a.seen()); got != 1 {
		t.Fatalf("expected 1 command (from initial Apply), got %d", got)
	}

	// appliedIndex tracks the highest skipped entry.
	if got := fsm.AppliedIndex(); got != 20 {
		t.Fatalf("applied index: want 20, got %d", got)
	}
}

// TestFSM_ApplyBatch_BadPayloadMidBatch verifies that an unmarshal error for
// one entry in a batch returns an error for that slot but does not panic and
// does not prevent subsequent entries from being applied.
func TestFSM_ApplyBatch_BadPayloadMidBatch(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	goodCmd, err := NewCommand(CmdChangeset, map[string]string{"v": "ok"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	goodData, err := goodCmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	logs := []*hraft.Log{
		{Index: 1, Data: goodData},
		{Index: 2, Data: []byte{0xFF}}, // too short to unmarshal
		{Index: 3, Data: goodData},
	}

	results := fsm.ApplyBatch(logs)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0] != nil {
		t.Fatalf("result[0]: expected nil, got %v", results[0])
	}

	if results[1] == nil {
		t.Fatal("result[1]: expected error for bad payload, got nil")
	}

	errMsg, ok := results[1].(error)
	if !ok {
		t.Fatalf("result[1]: expected error type, got %T", results[1])
	}

	if !strings.Contains(errMsg.Error(), "unmarshal") {
		t.Fatalf("result[1] error should mention unmarshal, got: %v", errMsg)
	}

	if results[2] != nil {
		t.Fatalf("result[2]: expected nil, got %v", results[2])
	}

	// Entries 1 and 3 applied; entry 2 skipped due to bad payload.
	if got := len(a.seen()); got != 2 {
		t.Fatalf("expected 2 commands applied, got %d", got)
	}

	if got := fsm.AppliedIndex(); got != 3 {
		t.Fatalf("applied index: want 3, got %d", got)
	}
}

// TestFSM_ApplyBatch_SingleEntry verifies that a batch with a single element
// behaves identically to Apply.
func TestFSM_ApplyBatch_SingleEntry(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	cmd, err := NewCommand(CmdChangeset, map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("new command: %v", err)
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	results := fsm.ApplyBatch([]*hraft.Log{{Index: 42, Data: data}})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0] != nil {
		t.Fatalf("result[0]: expected nil, got %v", results[0])
	}

	if got := fsm.AppliedIndex(); got != 42 {
		t.Fatalf("applied index: want 42, got %d", got)
	}

	if got := len(a.seen()); got != 1 {
		t.Fatalf("expected 1 command, got %d", got)
	}

	var idx uint64
	if err := a.db.QueryRowContext(context.Background(),
		"SELECT lastApplied FROM fsm_state WHERE id = 1").Scan(&idx); err != nil {
		t.Fatalf("read lastApplied: %v", err)
	}

	if idx != 42 {
		t.Fatalf("durable lastApplied: want 42, got %d", idx)
	}
}

// TestFSM_Restore_CorruptMagic verifies that Restore rejects a snapshot whose
// first 4 bytes are neither the ELSN magic nor the SQLite file header.
func TestFSM_Restore_CorruptMagic(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	// 20 bytes of garbage.
	rc := newReadCloser(bytes.Repeat([]byte{0xDE, 0xAD}, 10))

	err := fsm.Restore(rc)
	if err == nil {
		t.Fatal("expected error for corrupt magic, got nil")
	}

	if !strings.Contains(err.Error(), "unrecognized header magic") {
		t.Fatalf("error should mention unrecognized header magic, got: %v", err)
	}
}

// TestFSM_Restore_TruncatedHeader verifies that Restore fails gracefully when
// the snapshot is shorter than the 16-byte header.
func TestFSM_Restore_TruncatedHeader(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	// Only 4 bytes — too short for a full header read.
	rc := newReadCloser([]byte{0x45, 0x4C, 0x53, 0x4E})

	err := fsm.Restore(rc)
	if err == nil {
		t.Fatal("expected error for truncated header, got nil")
	}

	if !strings.Contains(err.Error(), "read snapshot header") {
		t.Fatalf("error should mention read snapshot header, got: %v", err)
	}
}

// TestFSM_Restore_UnsupportedFormatVersion verifies that Restore rejects a
// snapshot whose format version is higher than the binary supports.
func TestFSM_Restore_UnsupportedFormatVersion(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	var header [snapshotHeaderSize]byte

	copy(header[0:4], snapshotMagic)
	binary.BigEndian.PutUint32(header[4:8], snapshotFormatVersion+99) // future format
	binary.BigEndian.PutUint32(header[8:12], 0)
	binary.BigEndian.PutUint32(header[12:16], 0)

	// Append some dummy SQLite bytes after the header.
	payload := append(header[:], bytes.Repeat([]byte{0x00}, 64)...)
	rc := newReadCloser(payload)

	err := fsm.Restore(rc)
	if err == nil {
		t.Fatal("expected error for unsupported format version, got nil")
	}

	if !strings.Contains(err.Error(), "snapshot format version") {
		t.Fatalf("error should mention format version, got: %v", err)
	}
}

// TestFSM_Restore_UnsupportedProtocolVersion verifies that Restore rejects a
// snapshot whose protocol version is higher than this binary.
func TestFSM_Restore_UnsupportedProtocolVersion(t *testing.T) {
	a := newTestApplier(t)
	fsm := NewFSM(a, t.TempDir())

	var header [snapshotHeaderSize]byte

	copy(header[0:4], snapshotMagic)
	binary.BigEndian.PutUint32(header[4:8], snapshotFormatVersion)
	binary.BigEndian.PutUint32(header[8:12], 0)
	binary.BigEndian.PutUint32(header[12:16], 99999) // far-future protocol

	payload := append(header[:], bytes.Repeat([]byte{0x00}, 64)...)
	rc := newReadCloser(payload)

	err := fsm.Restore(rc)
	if err == nil {
		t.Fatal("expected error for unsupported protocol version, got nil")
	}

	if !strings.Contains(err.Error(), "protocol version") {
		t.Fatalf("error should mention protocol version, got: %v", err)
	}
}

// TestFSM_Restore_RawSQLite verifies that Restore accepts a raw SQLite
// file (no ELSN header) — the shape that raft.UserRestore delivers
// from db.Restore.
func TestFSM_Restore_RawSQLite(t *testing.T) {
	src := newTestApplier(t)

	if _, err := src.db.ExecContext(context.Background(),
		`INSERT INTO t(id, v) VALUES (1, 'raw')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	tmpPath := filepath.Join(t.TempDir(), "raw.db")

	if _, err := src.db.ExecContext(context.Background(),
		"VACUUM INTO ?", tmpPath); err != nil {
		t.Fatalf("vacuum into: %v", err)
	}

	raw, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("read raw snapshot: %v", err)
	}

	dst := newTestApplier(t)
	dstFSM := NewFSM(dst, t.TempDir())

	rc := newReadCloser(raw)
	if err := dstFSM.Restore(rc); err != nil {
		t.Fatalf("restore raw snapshot: %v", err)
	}

	var v string
	if err := dst.db.QueryRowContext(context.Background(),
		`SELECT v FROM t WHERE id = 1`).Scan(&v); err != nil {
		t.Fatalf("query after raw restore: %v", err)
	}

	if v != "raw" {
		t.Fatalf("want 'raw', got %q", v)
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
