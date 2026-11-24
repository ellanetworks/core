// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/google/uuid"
	"github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

const commandSchema = 31

var ErrInvalidCommand = errors.New("invalid command")

type invalidCommandError struct {
	err error
}

func (e *invalidCommandError) Error() string { return e.err.Error() }

func (e *invalidCommandError) Unwrap() error { return e.err }

func (e *invalidCommandError) Is(target error) bool { return target == ErrInvalidCommand }

func invalidCommand(err error) error {
	return &invalidCommandError{err: err}
}

type wireCommand struct {
	Op      string          `json:"o"`
	Payload json.RawMessage `json:"p"`
}

type commandBatch struct {
	LeaderTime int64         `json:"l"`
	Commands   []wireCommand `json:"c"`
}

func encodeCommandBatch(b commandBatch) ([]byte, error) {
	body, err := json.Marshal(b)
	if err != nil {
		return nil, fmt.Errorf("encode command batch: %w", err)
	}

	cmd := ellaraft.Command{Type: ellaraft.CmdBatch, Payload: body}

	return cmd.MarshalBinary()
}

type commandResult struct {
	value any
	err   error
}

type applyInfo struct {
	index      uint64
	term       uint64
	leaderTime int64
	schema     int
}

type applyInfoKey struct{}

func applyInfoFrom(ctx context.Context) (applyInfo, bool) {
	info, ok := ctx.Value(applyInfoKey{}).(applyInfo)

	return info, ok
}

func applyTime(ctx context.Context) time.Time {
	if info, ok := applyInfoFrom(ctx); ok {
		return time.Unix(0, info.leaderTime)
	}

	return time.Now()
}

func applyIndexFloor(ctx context.Context, db *Database) int64 {
	if info, ok := applyInfoFrom(ctx); ok {
		return int64(info.index)
	}

	if db.raftManager != nil {
		return int64(db.raftManager.AppliedIndex()) + 1
	}

	return 0
}

func payloadID(ctx context.Context, id, what string) (string, error) {
	if id != "" {
		return id, nil
	}

	if _, ok := applyInfoFrom(ctx); ok {
		return "", invalidCommand(fmt.Errorf("%s id is required", what))
	}

	generated, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("generate %s id: %w", what, err)
	}

	return generated.String(), nil
}

func newIDs(n int, what string) ([]string, error) {
	if n == 0 {
		return nil, nil
	}

	ids := make([]string, n)

	for i := range ids {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate %s id: %w", what, err)
		}

		ids[i] = id.String()
	}

	return ids, nil
}

func indexOr(ids []string, i int) string {
	if i < len(ids) {
		return ids[i]
	}

	return ""
}

func payloadNodeID(raw string) (pki.NodeID, error) {
	encoded, err := json.Marshal(pki.NodeID(raw))
	if err != nil {
		return "", invalidCommand(fmt.Errorf("encode node id: %w", err))
	}

	var id pki.NodeID
	if err := json.Unmarshal(encoded, &id); err != nil {
		return "", invalidCommand(err)
	}

	return id, nil
}

type fatalApplyError struct {
	index uint64
	err   error
}

func (e *fatalApplyError) Error() string {
	return fmt.Sprintf("apply entry %d: %v", e.index, e.err)
}

func (e *fatalApplyError) Unwrap() error { return e.err }

type commandHandler struct {
	minSchema int
	topics    []Topic
	payload   reflect.Type
	check     func(raw json.RawMessage) error
	apply     func(db *Database, ctx context.Context, raw json.RawMessage) (any, error)
}

func commandHandlerFor(name string) (commandHandler, bool) {
	if _, retired := retiredChangesetOps[name]; retired {
		return commandHandler{}, false
	}

	if h, ok := changesetOps[name]; ok {
		return commandHandler{minSchema: max(h.minSchema, commandSchema), topics: h.topics, payload: h.payload, check: h.checkJSON, apply: h.applyJSON}, true
	}

	if h, ok := intentOps[name]; ok && h.commandApply != nil {
		return commandHandler{minSchema: max(h.minSchema, commandSchema), topics: h.topics, payload: h.payload, check: h.commandCheck, apply: h.commandApply}, true
	}

	return commandHandler{}, false
}

func asCommand[P any](apply func(db *Database, ctx context.Context, p *P) (any, error)) OpOption {
	return func(m *opMeta) {
		m.commandApply = decodeAndApply(apply)
		m.commandCheck = checkPayload[P]
		m.payload = reflect.TypeFor[P]()
	}
}

func decodePayload[P any](raw json.RawMessage) (*P, error) {
	var p P

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("decode %T payload: %w", p, err)
	}

	if dec.More() {
		return nil, fmt.Errorf("decode %T payload: trailing data", p)
	}

	return &p, nil
}

func checkPayload[P any](raw json.RawMessage) error {
	if _, err := decodePayload[P](raw); err != nil {
		return invalidCommand(err)
	}

	return nil
}

func decodeAndApply[P any](apply func(db *Database, ctx context.Context, p *P) (any, error)) func(db *Database, ctx context.Context, raw json.RawMessage) (any, error) {
	return func(db *Database, ctx context.Context, raw json.RawMessage) (any, error) {
		info, ok := applyInfoFrom(ctx)
		if !ok {
			var p P
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, invalidCommand(fmt.Errorf("decode %T payload: %w", p, err))
			}

			return apply(db, ctx, &p)
		}

		p, err := decodePayload[P](raw)
		if err != nil {
			return nil, &fatalApplyError{index: info.index, err: err}
		}

		return apply(db, ctx, p)
	}
}

func withoutResult[P any](apply func(db *Database, ctx context.Context, p *P) error) func(db *Database, ctx context.Context, p *P) (any, error) {
	return func(db *Database, ctx context.Context, p *P) (any, error) {
		return nil, apply(db, ctx, p)
	}
}

func (db *Database) applyCreateCellPositionCommand(ctx context.Context, c *CellPosition) (any, error) {
	if err := db.checkCreateCellPosition(ctx, c); err != nil {
		return nil, err
	}

	return nil, db.applyPutCellPosition(ctx, c)
}

func (db *Database) applyUpdateCellPositionCommand(ctx context.Context, c *CellPosition) (any, error) {
	if err := db.checkUpdateCellPosition(ctx, c); err != nil {
		return nil, err
	}

	return nil, db.applyPutCellPosition(ctx, c)
}

func (db *Database) applyDeleteCellPositionCommand(ctx context.Context, p *stringPayload) (any, error) {
	if err := db.checkDeleteCellPosition(ctx, p); err != nil {
		return nil, err
	}

	return nil, db.applyDeleteCellPosition(ctx, p)
}

func (db *Database) applyDeleteAllDynamicLeasesCommand(ctx context.Context, _ *struct{}) (any, error) {
	return nil, db.applyDeleteAllDynamicLeases(ctx)
}

func (db *Database) runCommand(ctx context.Context, h commandHandler, op string, index uint64, raw json.RawMessage) (value any, err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}

		logger.DBLog.Error("command apply panicked", zap.String("operation", op), zap.Uint64("index", index), zap.Any("panic", r), zap.Stack("stack"))

		value = nil
		err = &fatalApplyError{index: index, err: fmt.Errorf("%s panicked: %v", op, r)}
	}()

	return h.apply(db, ctx, raw)
}

var fatalSQLiteCodes = map[sqlite3.ErrNo]struct{}{
	sqlite3.ErrInternal:  {},
	sqlite3.ErrPerm:      {},
	sqlite3.ErrAbort:     {},
	sqlite3.ErrBusy:      {},
	sqlite3.ErrLocked:    {},
	sqlite3.ErrNomem:     {},
	sqlite3.ErrReadonly:  {},
	sqlite3.ErrInterrupt: {},
	sqlite3.ErrIoErr:     {},
	sqlite3.ErrCorrupt:   {},
	sqlite3.ErrFull:      {},
	sqlite3.ErrCantOpen:  {},
	sqlite3.ErrProtocol:  {},
	sqlite3.ErrSchema:    {},
	sqlite3.ErrNoLFS:     {},
	sqlite3.ErrAuth:      {},
	sqlite3.ErrNotADB:    {},
}

func isFatalApplyError(err error) bool {
	var fatal *fatalApplyError
	if errors.As(err, &fatal) {
		return true
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, sql.ErrConnDone) || errors.Is(err, sql.ErrTxDone) || errors.Is(err, driver.ErrBadConn) {
		return true
	}

	var se sqlite3.Error
	if errors.As(err, &se) {
		_, ok := fatalSQLiteCodes[se.Code]

		return ok
	}

	return false
}

func (db *Database) commandMode(ctx context.Context) bool {
	return db.appliedSchemaAtLeast(ctx, commandSchema)
}

var observeApply func(db *Database, entries []ellaraft.Entry) func(out []any, err error)

func (db *Database) ApplyEntries(ctx context.Context, entries []ellaraft.Entry) ([]any, error) {
	if observeApply == nil {
		return db.applyEntries(ctx, entries)
	}

	done := observeApply(db, entries)
	out, err := db.applyEntries(ctx, entries)
	done(out, err)

	return out, err
}

func (db *Database) applyEntries(ctx context.Context, entries []ellaraft.Entry) ([]any, error) {
	batches := make([]commandBatch, len(entries))

	for i, e := range entries {
		if err := json.Unmarshal(e.Data, &batches[i]); err != nil {
			return nil, &fatalApplyError{index: e.Index, err: fmt.Errorf("decode command batch: %w", err)}
		}
	}

	out := make([]any, 0, len(entries))

	for len(entries) > 0 {
		n := verifyStateBoundary(batches)

		results, verified, err := db.applyEntryRun(ctx, entries[:n], batches[:n])
		if err != nil {
			return nil, err
		}

		out = append(out, results...)

		if verified != 0 {
			db.captureStateHash(verified)
		}

		entries, batches = entries[n:], batches[n:]
	}

	return out, nil
}

func verifyStateBoundary(batches []commandBatch) int {
	for i, b := range batches {
		for _, c := range b.Commands {
			if c.Op == verifyStateOp {
				return i + 1
			}
		}
	}

	return len(batches)
}

func (db *Database) applyEntryRun(ctx context.Context, entries []ellaraft.Entry, batches []commandBatch) ([]any, uint64, error) {
	conn, err := db.writer().Conn(ctx)
	if err != nil {
		return nil, 0, &fatalApplyError{index: entries[0].Index, err: fmt.Errorf("acquire sqlite conn: %w", err)}
	}

	defer func() { _ = conn.Close() }()

	out := make([]any, len(entries))

	var (
		topics         []Topic
		migrationCheck bool
		leaderTime     int64
		applied        bool
		verified       uint64
	)

	err = conn.Raw(func(raw any) error {
		sqliteConn, err := rawSQLiteConn(raw)
		if err != nil {
			return &fatalApplyError{index: entries[0].Index, err: err}
		}

		dconn, ok := raw.(driver.Conn)
		if !ok {
			return &fatalApplyError{index: entries[0].Index, err: errors.New("raw sqlite conn does not implement driver.Conn")}
		}

		exec := func(stmt string, args ...driver.NamedValue) error {
			_, err := sqliteConn.ExecContext(ctx, stmt, args)
			return err
		}

		if err := exec("PRAGMA foreign_keys = ON"); err != nil {
			return &fatalApplyError{index: entries[0].Index, err: fmt.Errorf("enable foreign keys: %w", err)}
		}

		if err := exec("BEGIN IMMEDIATE"); err != nil {
			return &fatalApplyError{index: entries[0].Index, err: fmt.Errorf("begin: %w", err)}
		}

		committed := false

		defer func() {
			if !committed {
				_, _ = sqliteConn.ExecContext(context.Background(), "ROLLBACK", nil)
			}
		}()

		pinned := sql.OpenDB(&pinnedConnector{conn: dconn})
		pinned.SetMaxOpenConns(1)
		pinned.SetMaxIdleConns(1)

		defer func() { _ = pinned.Close() }()

		var lastApplied uint64

		if err := pinned.QueryRowContext(ctx, "SELECT lastApplied, leaderTime FROM fsm_state WHERE id = 1").Scan(&lastApplied, &leaderTime); err != nil {
			return &fatalApplyError{index: entries[0].Index, err: fmt.Errorf("read fsm_state: %w", err)}
		}

		schema, err := readSchemaVersion(ctx, pinned)
		if err != nil {
			return &fatalApplyError{index: entries[0].Index, err: err}
		}

		baseCtx := context.WithValue(ctx, pinnedRunnerCtxKey{}, pinnedQuerier{db: pinned})

		var lastIndex, lastTerm uint64

		for i, e := range entries {
			if e.Index <= lastApplied {
				continue
			}

			b := batches[i]
			leaderTime = max(leaderTime, b.LeaderTime)
			cmdCtx := context.WithValue(baseCtx, applyInfoKey{}, applyInfo{index: e.Index, term: e.Term, leaderTime: leaderTime, schema: schema})
			results := make([]commandResult, len(b.Commands))

			for j, c := range b.Commands {
				h, ok := commandHandlerFor(c.Op)
				if !ok {
					return &fatalApplyError{index: e.Index, err: fmt.Errorf("command %d: unknown operation %q", j, c.Op)}
				}

				if h.minSchema > schema {
					results[j] = commandResult{err: fmt.Errorf("%w: %s requires schema %d, applied %d", ErrMigrationPending, c.Op, h.minSchema, schema)}

					continue
				}

				if err := exec("SAVEPOINT command"); err != nil {
					return &fatalApplyError{index: e.Index, err: fmt.Errorf("command %d (%s): savepoint: %w", j, c.Op, err)}
				}

				value, applyErr := db.runCommand(cmdCtx, h, c.Op, e.Index, c.Payload)
				if applyErr != nil {
					if isFatalApplyError(applyErr) {
						return &fatalApplyError{index: e.Index, err: fmt.Errorf("command %d (%s): %w", j, c.Op, applyErr)}
					}

					if err := exec("ROLLBACK TO command"); err != nil {
						return &fatalApplyError{index: e.Index, err: fmt.Errorf("command %d (%s): rollback to savepoint: %w", j, c.Op, err)}
					}
				}

				if err := exec("RELEASE command"); err != nil {
					return &fatalApplyError{index: e.Index, err: fmt.Errorf("command %d (%s): release savepoint: %w", j, c.Op, err)}
				}

				if applyErr != nil {
					results[j] = commandResult{err: applyErr}

					continue
				}

				results[j] = commandResult{value: value}

				topics = append(topics, h.topics...)

				if c.Op == "UpsertClusterMember" {
					migrationCheck = true
				}

				if c.Op == verifyStateOp {
					verified = e.Index
				}
			}

			out[i] = results
			lastIndex, lastTerm = e.Index, e.Term
		}

		if lastIndex == 0 {
			return nil
		}

		if err := exec("UPDATE fsm_state SET lastApplied = ?, lastAppliedTerm = ?, leaderTime = ? WHERE id = 1",
			driver.NamedValue{Ordinal: 1, Value: int64(lastIndex)},
			driver.NamedValue{Ordinal: 2, Value: int64(lastTerm)},
			driver.NamedValue{Ordinal: 3, Value: leaderTime}); err != nil {
			return &fatalApplyError{index: lastIndex, err: fmt.Errorf("update fsm_state: %w", err)}
		}

		if err := exec("COMMIT"); err != nil {
			return &fatalApplyError{index: lastIndex, err: fmt.Errorf("commit: %w", err)}
		}

		committed = true
		applied = true

		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	if !applied {
		return out, 0, nil
	}

	db.raiseAppliedLeaderTime(leaderTime)

	if migrationCheck {
		db.signalMigrationCheck()
	}

	db.publishOpTopics(uniqueTopics(topics))

	return out, verified, nil
}

func uniqueTopics(in []Topic) []Topic {
	seen := make(map[Topic]struct{}, len(in))
	out := make([]Topic, 0, len(in))

	for _, t := range in {
		if _, ok := seen[t]; ok {
			continue
		}

		seen[t] = struct{}{}
		out = append(out, t)
	}

	return out
}

func (db *Database) raiseAppliedLeaderTime(t int64) {
	for {
		current := db.appliedLeaderTime.Load()
		if t <= current || db.appliedLeaderTime.CompareAndSwap(current, t) {
			return
		}
	}
}

func (db *Database) loadAppliedLeaderTime(ctx context.Context) error {
	var t int64
	if err := db.writer().QueryRowContext(ctx, "SELECT leaderTime FROM fsm_state WHERE id = 1").Scan(&t); err != nil {
		return fmt.Errorf("read fsm_state.leaderTime: %w", err)
	}

	db.raiseAppliedLeaderTime(t)

	return nil
}

const leaderTimeSkewWarning = 5 * time.Second

func (db *Database) nextLeaderTime(last int64) int64 {
	now := time.Now().UnixNano()
	next := max(last+1, now, db.appliedLeaderTime.Load()+1)

	if ahead := time.Duration(next - now); ahead > leaderTimeSkewWarning {
		logger.DBLog.Warn("Replicated time is ahead of this leader's clock", zap.Duration("ahead", ahead))
	}

	return next
}

func (db *Database) proposeCommand(ctx context.Context, op string, payload json.RawMessage) (_ *ellaraft.ProposeResult, err error) {
	ctx, span := db.startProposeSpan(ctx, op)

	defer func() { endSpan(span, err) }()

	if h, ok := changesetOps[op]; ok && h.unchanged != nil {
		if value, skip, err := h.unchanged(db, ctx, payload); err == nil && skip {
			return &ellaraft.ProposeResult{Value: value}, nil
		}
	}

	var (
		result commandResult
		index  uint64
	)

	var batcher *commandBatcher

	batcher, err = db.commandBatcher()
	if err == nil {
		result, index, err = batcher.propose(wireCommand{Op: op, Payload: payload})
	}

	if err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.Int64("ella.raft.index", int64(index)))

	if result.err != nil {
		return nil, result.err
	}

	return &ellaraft.ProposeResult{Value: result.value, Index: index}, nil
}

type localProposer struct {
	mu sync.Mutex
}

func (l *localProposer) apply(db *Database, body []byte) (*ellaraft.ProposeResult, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	ctx := context.Background()

	var index, term uint64
	if err := db.writer().QueryRowContext(ctx, "SELECT lastApplied, lastAppliedTerm FROM fsm_state WHERE id = 1").Scan(&index, &term); err != nil {
		return nil, fmt.Errorf("read fsm_state: %w", err)
	}

	index++

	out, err := db.ApplyEntries(ctx, []ellaraft.Entry{{Index: index, Term: max(term, 1), Data: body}})
	if err != nil {
		return nil, err
	}

	return &ellaraft.ProposeResult{Value: out[0], Index: index}, nil
}
