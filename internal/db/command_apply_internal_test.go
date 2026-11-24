// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	hraft "github.com/hashicorp/raft"
	"github.com/mattn/go-sqlite3"
)

func useChangesetPath(t *testing.T, database *Database) {
	t.Helper()

	atChangesetSchema(t, database)
}

func newCommandTestDB(t *testing.T) *Database {
	t.Helper()

	database, err := NewDatabaseWithoutRaft(t.Context(), filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = database.Close() })

	return database
}

func registerTestCommand(t *testing.T, name string, minSchema int, apply func(db *Database, ctx context.Context, p *json.RawMessage) (any, error)) {
	t.Helper()

	if _, exists := changesetOps[name]; exists {
		t.Fatalf("test command %s is already registered", name)
	}

	changesetOps[name] = changesetOpHandler{minSchema: minSchema, payload: reflect.TypeFor[json.RawMessage](), checkJSON: checkPayload[json.RawMessage], applyJSON: decodeAndApply(apply)}

	t.Cleanup(func() { delete(changesetOps, name) })
}

func command(t *testing.T, op string, payload any) wireCommand {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal %s payload: %v", op, err)
	}

	return wireCommand{Op: op, Payload: raw}
}

func entry(t *testing.T, index uint64, leaderTime int64, cmds ...wireCommand) ellaraft.Entry {
	t.Helper()

	data, err := json.Marshal(commandBatch{LeaderTime: leaderTime, Commands: cmds})
	if err != nil {
		t.Fatalf("marshal batch: %v", err)
	}

	return ellaraft.Entry{Index: index, Term: 1, Data: data}
}

func nextIndex(t *testing.T, database *Database) uint64 {
	t.Helper()

	return fsmState(t, database).lastApplied + 1
}

type testFSMState struct {
	lastApplied uint64
	term        uint64
	leaderTime  int64
}

func fsmState(t *testing.T, database *Database) testFSMState {
	t.Helper()

	var s testFSMState
	if err := database.writer().QueryRowContext(t.Context(), "SELECT lastApplied, lastAppliedTerm, leaderTime FROM fsm_state WHERE id = 1").Scan(&s.lastApplied, &s.term, &s.leaderTime); err != nil {
		t.Fatalf("read fsm_state: %v", err)
	}

	return s
}

func resultsOf(t *testing.T, out []any, i int) []commandResult {
	t.Helper()

	results, ok := out[i].([]commandResult)
	if !ok {
		t.Fatalf("entry %d result = %T, want []commandResult", i, out[i])
	}

	return results
}

func profileExists(t *testing.T, database *Database, name string) bool {
	t.Helper()

	_, err := database.GetProfile(t.Context(), name)
	if err != nil && !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetProfile(%s): %v", name, err)
	}

	return err == nil
}

func requireFatal(t *testing.T, err error) {
	t.Helper()

	var fatal *fatalApplyError
	if !errors.As(err, &fatal) {
		t.Fatalf("err = %v, want a fatal apply error", err)
	}
}

func TestApplyEntriesIsolatesAFailingCommand(t *testing.T) {
	database := newCommandTestDB(t)

	registerTestCommand(t, "TestWriteThenReject", 1, func(db *Database, ctx context.Context, p *json.RawMessage) (any, error) {
		if _, err := db.applyCreateProfile(ctx, newTestProfile(t, "rejected")); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("%w: rejected after a write", ErrInUse)
	})

	first := newTestProfile(t, "first")
	duplicate := newTestProfile(t, "first")
	last := newTestProfile(t, "last")
	index := nextIndex(t, database)

	out, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{entry(t, index, 1,
		command(t, "CreateProfile", first),
		command(t, "TestWriteThenReject", nil),
		command(t, "CreateProfile", duplicate),
		command(t, "CreateProfile", last),
	)})
	if err != nil {
		t.Fatalf("ApplyEntries: %v", err)
	}

	results := resultsOf(t, out, 0)

	if results[0].err != nil || results[3].err != nil {
		t.Fatalf("results = %+v, want the first and last commands to succeed", results)
	}

	if !errors.Is(results[1].err, ErrInUse) {
		t.Fatalf("rejected command result = %v, want ErrInUse", results[1].err)
	}

	if !errors.Is(results[2].err, ErrAlreadyExists) {
		t.Fatalf("duplicate command result = %v, want ErrAlreadyExists", results[2].err)
	}

	if !profileExists(t, database, "first") || !profileExists(t, database, "last") {
		t.Fatal("the commands next to the failing ones must commit")
	}

	if profileExists(t, database, "rejected") {
		t.Fatal("a rejected command must leave no trace")
	}

	if got := fsmState(t, database).lastApplied; got != index {
		t.Fatalf("lastApplied = %d, want %d", got, index)
	}
}

func TestApplyEntriesHaltsOnUnknownOperation(t *testing.T) {
	database := newCommandTestDB(t)
	before := fsmState(t, database)

	_, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{entry(t, before.lastApplied+1, 1,
		command(t, "CreateProfile", newTestProfile(t, "before-unknown")),
		wireCommand{Op: "NoSuchOperation", Payload: json.RawMessage(`{}`)},
	)})
	requireFatal(t, err)

	if profileExists(t, database, "before-unknown") {
		t.Fatal("a fatal entry must roll back the whole transaction")
	}

	if after := fsmState(t, database); after != before {
		t.Fatalf("fsm_state = %+v after a fatal entry, want %+v", after, before)
	}
}

func TestApplyEntriesHaltsOnRetiredOperation(t *testing.T) {
	database := newCommandTestDB(t)

	_, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{entry(t, nextIndex(t, database), 1,
		wireCommand{Op: "InsertAuditLog", Payload: json.RawMessage(`{}`)},
	)})
	requireFatal(t, err)
}

func requireMigrationPending(t *testing.T, out []any, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("ApplyEntries: %v", err)
	}

	if got := resultsOf(t, out, 0)[0].err; !errors.Is(got, ErrMigrationPending) {
		t.Fatalf("result = %v, want ErrMigrationPending", got)
	}
}

func TestApplyEntriesRejectsOperationAboveTheAppliedSchema(t *testing.T) {
	database := newCommandTestDB(t)

	registerTestCommand(t, "TestFutureOperation", SchemaVersion()+1, func(*Database, context.Context, *json.RawMessage) (any, error) {
		return nil, nil
	})

	out, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{entry(t, nextIndex(t, database), 1,
		wireCommand{Op: "TestFutureOperation", Payload: json.RawMessage(`null`)},
	)})
	requireMigrationPending(t, out, err)
}

func TestApplyEntriesRequiresTheCommandSchema(t *testing.T) {
	database := newDatabaseAtVersion(t, commandSchema-1)

	out, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{entry(t, 1, 1,
		command(t, "SetRetentionPolicy", &RetentionPolicy{Category: CategoryAuditLogs, Days: 3}),
	)})
	requireMigrationPending(t, out, err)
}

func TestApplyEntriesReturnsAnUnclassifiedErrorAsTheResult(t *testing.T) {
	database := newCommandTestDB(t)
	index := nextIndex(t, database)

	registerTestCommand(t, "TestWriteThenFail", 1, func(db *Database, ctx context.Context, p *json.RawMessage) (any, error) {
		if _, err := db.applyCreateProfile(ctx, newTestProfile(t, "partial")); err != nil {
			return nil, err
		}

		return nil, errors.New("not allowed")
	})

	out, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{
		entry(t, index, 10, command(t, "CreateProfile", newTestProfile(t, "earlier-entry"))),
		entry(t, index+1, 20, wireCommand{Op: "TestWriteThenFail", Payload: json.RawMessage(`null`)}),
	})
	if err != nil {
		t.Fatalf("ApplyEntries: %v", err)
	}

	if got := resultsOf(t, out, 1)[0].err; got == nil || got.Error() != "not allowed" {
		t.Fatalf("result = %v, want the apply function's error", got)
	}

	if !profileExists(t, database, "earlier-entry") || profileExists(t, database, "partial") {
		t.Fatal("the failing command must roll back alone")
	}

	if got := fsmState(t, database).lastApplied; got != index+1 {
		t.Fatalf("lastApplied = %d, want %d", got, index+1)
	}
}

func TestApplyEntriesHaltsOnAStorageError(t *testing.T) {
	database := newCommandTestDB(t)
	before := fsmState(t, database)

	registerTestCommand(t, "TestStorageFailure", 1, func(db *Database, ctx context.Context, p *json.RawMessage) (any, error) {
		if _, err := db.applyCreateProfile(ctx, newTestProfile(t, "partial")); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("write: %w", sqlite3.Error{Code: sqlite3.ErrIoErr})
	})

	_, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{
		entry(t, before.lastApplied+1, 10, command(t, "CreateProfile", newTestProfile(t, "earlier-entry"))),
		entry(t, before.lastApplied+2, 20, wireCommand{Op: "TestStorageFailure", Payload: json.RawMessage(`null`)}),
	})
	requireFatal(t, err)

	for _, name := range []string{"earlier-entry", "partial"} {
		if profileExists(t, database, name) {
			t.Fatalf("profile %s survived a fatal batch", name)
		}
	}

	if after := fsmState(t, database); after != before {
		t.Fatalf("fsm_state = %+v after a fatal batch, want %+v", after, before)
	}
}

func TestApplyEntriesHaltsOnAnUndecodablePayload(t *testing.T) {
	for name, payload := range map[string]string{
		"wrong type":    `"not a profile"`,
		"unknown field": `{"id":"p","name":"p","fieldFromANewerRelease":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			database := newCommandTestDB(t)

			_, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{entry(t, nextIndex(t, database), 1,
				wireCommand{Op: "CreateProfile", Payload: json.RawMessage(payload)},
			)})
			requireFatal(t, err)
		})
	}
}

func TestApplyEntriesSkipsAppliedEntries(t *testing.T) {
	database := newCommandTestDB(t)
	index := nextIndex(t, database)

	applied := entry(t, index, 1, command(t, "CreateProfile", newTestProfile(t, "once")))

	if _, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{applied}); err != nil {
		t.Fatalf("ApplyEntries: %v", err)
	}

	out, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{
		applied,
		entry(t, index+1, 2, command(t, "CreateProfile", newTestProfile(t, "twice"))),
	})
	if err != nil {
		t.Fatalf("replay: %v", err)
	}

	if out[0] != nil {
		t.Fatalf("replayed entry result = %+v, want nil", out[0])
	}

	if got := resultsOf(t, out, 1)[0].err; got != nil {
		t.Fatalf("new entry result = %v", got)
	}

	if got := fsmState(t, database); got.lastApplied != index+1 || got.term != 1 {
		t.Fatalf("fsm_state = %+v, want lastApplied %d term 1", got, index+1)
	}
}

func TestApplyEntriesKeepsLeaderTimeMonotonic(t *testing.T) {
	database := newCommandTestDB(t)

	var seen []applyInfo

	registerTestCommand(t, "TestRecordApplyInfo", 1, func(_ *Database, ctx context.Context, _ *json.RawMessage) (any, error) {
		info, ok := applyInfoFrom(ctx)
		if !ok {
			return nil, errors.New("no apply info")
		}

		seen = append(seen, info)

		return nil, nil
	})

	record := wireCommand{Op: "TestRecordApplyInfo", Payload: json.RawMessage(`null`)}
	index := nextIndex(t, database)
	base := fsmState(t, database).leaderTime + 1000

	if _, err := database.ApplyEntries(t.Context(), []ellaraft.Entry{
		entry(t, index, base+100, record),
		entry(t, index+1, base+50, record),
	}); err != nil {
		t.Fatalf("ApplyEntries: %v", err)
	}

	want := []applyInfo{
		{index: index, term: 1, leaderTime: base + 100, schema: SchemaVersion()},
		{index: index + 1, term: 1, leaderTime: base + 100, schema: SchemaVersion()},
	}

	if len(seen) != len(want) || seen[0] != want[0] || seen[1] != want[1] {
		t.Fatalf("apply info = %+v, want %+v", seen, want)
	}

	if got := fsmState(t, database).leaderTime; got != base+100 {
		t.Fatalf("persisted leaderTime = %d, want %d", got, base+100)
	}

	if got := database.nextLeaderTime(0); got <= base+100 {
		t.Fatalf("next leader time %d does not exceed the applied %d", got, base+100)
	}

	if err := database.Reopen(t.Context()); err != nil {
		t.Fatalf("Reopen: %v", err)
	}

	if got := database.appliedLeaderTime.Load(); got != base+100 {
		t.Fatalf("leader time after reopen = %d, want %d", got, base+100)
	}
}

func TestApplyEntriesRunsForeignKeyCascades(t *testing.T) {
	database := newCommandTestDB(t)
	ctx := t.Context()

	if _, err := database.writer().ExecContext(ctx, "INSERT INTO users (id, email, roleID, hashedPassword) VALUES ('u1', 'cascade@example.com', 1, 'x')"); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	if _, err := database.writer().ExecContext(ctx, "INSERT INTO api_tokens (id, token_id, name, token_hash, user_id) VALUES ('t1', 'tid', 'token', 'hash', 'u1')"); err != nil {
		t.Fatalf("insert token: %v", err)
	}

	if _, err := database.writer().ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}

	registerTestCommand(t, "TestDeleteUserRow", 1, func(db *Database, ctx context.Context, _ *json.RawMessage) (any, error) {
		_, err := db.runner(ctx).ExecContext(ctx, "DELETE FROM users WHERE id = 'u1'")
		return nil, err
	})

	if _, err := database.ApplyEntries(ctx, []ellaraft.Entry{entry(t, nextIndex(t, database), 1,
		wireCommand{Op: "TestDeleteUserRow", Payload: json.RawMessage(`null`)},
	)}); err != nil {
		t.Fatalf("ApplyEntries: %v", err)
	}

	var tokens int
	if err := database.writer().QueryRowContext(ctx, "SELECT COUNT(*) FROM api_tokens").Scan(&tokens); err != nil {
		t.Fatalf("count tokens: %v", err)
	}

	if tokens != 0 {
		t.Fatalf("%d api tokens survived their user: the cascade did not run", tokens)
	}
}

func TestLocalWritesUseCommandsFromTheCommandSchema(t *testing.T) {
	database := newDatabaseAtVersion(t, commandSchema-1)
	ctx := t.Context()

	if err := database.SetRetentionPolicy(ctx, &RetentionPolicy{Category: CategoryAuditLogs, Days: 3}); err != nil {
		t.Fatalf("SetRetentionPolicy at schema %d: %v", commandSchema-1, err)
	}

	if got := fsmState(t, database); got.lastApplied != 0 || got.leaderTime != 0 {
		t.Fatalf("fsm_state = %+v after a write below the command schema, want it untouched", got)
	}

	if err := runMigrations(ctx, database.connection, commandSchema); err != nil {
		t.Fatalf("migrate to %d: %v", commandSchema, err)
	}

	database.appliedSchemaCache.Store(0)

	if err := database.SetRetentionPolicy(ctx, &RetentionPolicy{Category: CategoryAuditLogs, Days: 4}); err != nil {
		t.Fatalf("SetRetentionPolicy at schema %d: %v", commandSchema, err)
	}

	if got := fsmState(t, database); got.lastApplied != 1 || got.leaderTime == 0 {
		t.Fatalf("fsm_state = %+v after a command, want lastApplied 1 and a leader time", got)
	}

	days, err := database.GetRetentionPolicy(ctx, CategoryAuditLogs)
	if err != nil || days != 4 {
		t.Fatalf("retention = %d, %v; want 4", days, err)
	}
}

func TestRaftWritesUseChangesetsBelowTheCommandSchema(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := t.Context()

	useChangesetPath(t, database)

	before := fsmState(t, database)

	if err := database.SetRetentionPolicy(ctx, &RetentionPolicy{Category: CategoryAuditLogs, Days: 5}); err != nil {
		t.Fatalf("SetRetentionPolicy: %v", err)
	}

	after := fsmState(t, database)
	if after.lastApplied <= before.lastApplied || after.leaderTime != before.leaderTime {
		t.Fatalf("fsm_state %+v -> %+v: a changeset write must advance lastApplied without a leader time", before, after)
	}

	setSchemaVersion(t, database, SchemaVersion())

	if err := database.SetRetentionPolicy(ctx, &RetentionPolicy{Category: CategoryAuditLogs, Days: 6}); err != nil {
		t.Fatalf("SetRetentionPolicy: %v", err)
	}

	if got := fsmState(t, database); got.leaderTime <= after.leaderTime {
		t.Fatalf("fsm_state %+v -> %+v: a command write must stamp a leader time", after, got)
	}
}

func TestForwardedOperationsUseTheCommandPath(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := t.Context()

	position := &CellPosition{ID: "cell-1", RAT: RATNR, Mcc: "001", Mnc: "01", CellIdentity: "0000a0001", Source: "provisioned", CreatedAt: "t", UpdatedAt: "t"}

	payload, err := json.Marshal(position)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	before := fsmState(t, database)

	if _, err := database.ApplyForwardedOperation(ctx, "CreateCellPosition", payload); err != nil {
		t.Fatalf("forwarded CreateCellPosition: %v", err)
	}

	if after := fsmState(t, database); after.leaderTime <= before.leaderTime {
		t.Fatalf("fsm_state %+v -> %+v: the forwarded write did not go through a command", before, after)
	}

	_, err = database.ApplyForwardedOperation(ctx, "CreateCellPosition", payload)
	if !errors.Is(err, ErrAlreadyExists) || ForwardCodeFor(err) != ellaraft.ForwardCodeAlreadyExists {
		t.Fatalf("duplicate forwarded CreateCellPosition: err = %v, want ErrAlreadyExists with its forward code", err)
	}

	sessions, err := json.Marshal(int64Payload{Value: time.Now().Unix()})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	result, err := database.ApplyForwardedOperation(ctx, "DeleteExpiredSessions", sessions)
	if err != nil {
		t.Fatalf("forwarded DeleteExpiredSessions: %v", err)
	}

	if count, ok := result.Value.(int); !ok || count != 0 {
		t.Fatalf("forwarded DeleteExpiredSessions result = %#v, want int 0", result.Value)
	}
}

func TestConcurrentCommandsShareRaftEntries(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := t.Context()

	const writers = 200

	before := database.RaftAppliedIndex()

	var wg sync.WaitGroup

	errs := make(chan error, writers)

	for i := range writers {
		wg.Go(func() {
			errs <- database.CreateProfile(ctx, newTestProfile(t, fmt.Sprintf("concurrent-%d", i)))
		})
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("CreateProfile: %v", err)
		}
	}

	if entries := database.RaftAppliedIndex() - before; entries >= writers {
		t.Fatalf("%d writes took %d raft entries, want them batched", writers, entries)
	}

	for i := range writers {
		if !profileExists(t, database, fmt.Sprintf("concurrent-%d", i)) {
			t.Fatalf("profile concurrent-%d is missing", i)
		}
	}
}

func TestCommandBatcherRejectsWritesWhenTheQueueIsFull(t *testing.T) {
	b := &commandBatcher{queue: make(chan *commandRequest)}

	if _, _, err := b.propose(wireCommand{Op: "CreateProfile"}); !errors.Is(err, ErrProposeTimeout) {
		t.Fatalf("a full queue must return a retryable error, got %v", err)
	}
}

func TestCommandBatcherFailsQueuedWritesOnClose(t *testing.T) {
	database := newStandaloneDB(t)

	b, err := database.commandBatcher()
	if err != nil {
		t.Fatalf("commandBatcher: %v", err)
	}

	b.close()

	if _, _, err := b.propose(wireCommand{Op: "CreateProfile"}); !errors.Is(err, ErrProposeTimeout) {
		t.Fatalf("a closed batcher must return a retryable error, got %v", err)
	}
}

func TestPausedCommandsReturnARetryableError(t *testing.T) {
	database := newStandaloneDB(t)

	b, err := database.commandBatcher()
	if err != nil {
		t.Fatalf("commandBatcher: %v", err)
	}

	resume := database.pauseCommands()

	if _, _, err := b.propose(wireCommand{Op: "CreateProfile"}); !errors.Is(err, ErrProposeTimeout) {
		t.Fatalf("the batcher in use when commands paused must return a retryable error, got %v", err)
	}

	if _, err := database.commandBatcher(); !errors.Is(err, ErrProposeTimeout) {
		t.Fatalf("commandBatcher while paused: err = %v, want a retryable error", err)
	}

	resume()

	if _, err := database.commandBatcher(); err != nil {
		t.Fatalf("commandBatcher after resuming: %v", err)
	}
}

func TestFSMAppliesInterleavedLegacyAndCommandEntries(t *testing.T) {
	database := newCommandTestDB(t)
	fsm := ellaraft.NewFSM(database, t.TempDir())
	start := nextIndex(t, database)

	batch := func(index uint64, name string) *hraft.Log {
		e := entry(t, index, int64(index), command(t, "CreateProfile", newTestProfile(t, name)))

		data, err := encodeCommandBatch(commandBatch{})
		if err != nil {
			t.Fatalf("encode: %v", err)
		}

		return &hraft.Log{Index: index, Term: 3, Type: hraft.LogCommand, Data: append(data[:2], e.Data...)}
	}

	intent := func(index uint64) *hraft.Log {
		cmd, err := ellaraft.NewCommand(ellaraft.CmdDeleteExpiredSessions, int64Payload{Value: time.Now().Unix()})
		if err != nil {
			t.Fatalf("NewCommand: %v", err)
		}

		data, err := cmd.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary: %v", err)
		}

		return &hraft.Log{Index: index, Term: 3, Type: hraft.LogCommand, Data: data}
	}

	results := fsm.ApplyBatch([]*hraft.Log{
		batch(start, "interleaved-1"),
		batch(start+1, "interleaved-2"),
		intent(start + 2),
		{Index: start + 3, Term: 3, Type: hraft.LogNoop},
		batch(start+4, "interleaved-3"),
		intent(start + 5),
	})

	for _, i := range []int{0, 1, 4} {
		if r := resultsOf(t, results, i); len(r) != 1 || r[0].err != nil {
			t.Fatalf("command entry %d result = %+v", i, r)
		}
	}

	if count, ok := results[2].(int); !ok || count != 0 {
		t.Fatalf("intent result = %#v, want 0", results[2])
	}

	for _, name := range []string{"interleaved-1", "interleaved-2", "interleaved-3"} {
		if !profileExists(t, database, name) {
			t.Fatalf("profile %s missing", name)
		}
	}

	if got := fsmState(t, database); got.lastApplied != start+5 || got.term != 3 {
		t.Fatalf("fsm_state = %+v, want lastApplied %d term 3", got, start+5)
	}

	if got := fsm.AppliedIndex(); got != start+5 {
		t.Fatalf("applied index = %d, want %d", got, start+5)
	}

	results = fsm.ApplyBatch([]*hraft.Log{
		batch(start+5, "replayed"),
		intent(start + 6),
		batch(start+7, "interleaved-4"),
	})

	if results[0] != nil || profileExists(t, database, "replayed") {
		t.Fatal("an entry at or below lastApplied must not apply")
	}

	if got := fsmState(t, database); got.lastApplied != start+7 {
		t.Fatalf("lastApplied = %d, want %d", got.lastApplied, start+7)
	}
}

func TestFSMWaitAppliedWakesOnApply(t *testing.T) {
	database := newCommandTestDB(t)
	fsm := ellaraft.NewFSM(database, t.TempDir())
	index := nextIndex(t, database)

	done := make(chan error, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		done <- fsm.WaitApplied(ctx, index)
	}()

	e := entry(t, index, 1, command(t, "CreateProfile", newTestProfile(t, "wake")))

	header, err := encodeCommandBatch(commandBatch{})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	fsm.ApplyBatch([]*hraft.Log{{Index: index, Term: 1, Type: hraft.LogCommand, Data: append(header[:2], e.Data...)}})

	if err := <-done; err != nil {
		t.Fatalf("WaitApplied: %v", err)
	}
}
