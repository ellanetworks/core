// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql/driver"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	changesetBatchMinSchema = 26
	groupCommitMaxWrites    = 256
	groupCommitEntryFraming = 64
)

var (
	groupCommitMaxEntryBytes = 512 << 10
	groupCommitMaxQueue      = 4096
	groupCommitCaptureBudget = 10 * time.Millisecond

	errWriteQueueFull = fmt.Errorf("%w: replicated write queue is full", ErrProposeTimeout)
)

type pendingWrite struct {
	ctx       context.Context
	operation string
	minSchema int
	applyFn   func(context.Context) (any, error)
	waitSpan  trace.Span
	done      chan writeOutcome
	once      sync.Once
}

type writeOutcome struct {
	result *ellaraft.ProposeResult
	err    error
}

type groupCommitter struct {
	mu      sync.Mutex
	pending []*pendingWrite
	running bool
}

type capturedWrite struct {
	write          *pendingWrite
	changeset      []byte
	value          any
	err            error
	dependsOnBatch bool
}

func (db *Database) submitWrite(ctx context.Context, operation string, minSchema int, applyFn func(context.Context) (any, error)) (*ellaraft.ProposeResult, error) {
	w := &pendingWrite{
		ctx:       ctx,
		operation: operation,
		minSchema: minSchema,
		applyFn:   applyFn,
		done:      make(chan writeOutcome, 1),
	}

	_, w.waitSpan = tracer.Start(ctx, "db/propose_lock_wait", trace.WithSpanKind(trace.SpanKindInternal))

	gc := &db.groupCommit

	gc.mu.Lock()

	if len(gc.pending) >= groupCommitMaxQueue {
		gc.mu.Unlock()
		endSpan(w.waitSpan, errWriteQueueFull)

		return nil, errWriteQueueFull
	}

	gc.pending = append(gc.pending, w)

	if !gc.running {
		gc.running = true

		go db.drainWrites()
	}

	gc.mu.Unlock()

	out := <-w.done

	return out.result, out.err
}

func (db *Database) drainWrites() {
	gc := &db.groupCommit

	for {
		gc.mu.Lock()

		if len(gc.pending) == 0 {
			gc.running = false
			gc.mu.Unlock()

			return
		}

		n := min(len(gc.pending), groupCommitMaxWrites)
		batch := gc.pending[:n:n]

		if n == len(gc.pending) {
			gc.pending = nil
		} else {
			gc.pending = gc.pending[n:]
		}

		gc.mu.Unlock()

		carry := db.commitWritesRecovering(batch)
		if len(carry) == 0 {
			continue
		}

		gc.mu.Lock()
		gc.pending = append(carry, gc.pending...)
		gc.mu.Unlock()
	}
}

func (w *pendingWrite) finish(result *ellaraft.ProposeResult, err error) {
	w.once.Do(func() {
		w.done <- writeOutcome{result: result, err: err}
	})
}

func (db *Database) commitWritesRecovering(batch []*pendingWrite) (carry []*pendingWrite) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}

		logger.DBLog.Error("replicated write batch panicked", zap.Any("panic", r), zap.Stack("stack"))

		err := fmt.Errorf("replicated write batch panicked: %v", r)
		for _, w := range batch {
			w.finish(nil, err)
		}

		carry = nil
	}()

	return db.commitWrites(batch)
}

func recoverApply(operation string, applyFn func(context.Context) (any, error)) func(context.Context) (any, error) {
	return func(ctx context.Context) (value any, err error) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}

			logger.DBLog.Error("replicated write panicked", zap.String("operation", operation), zap.Any("panic", r), zap.Stack("stack"))

			value = nil
			err = fmt.Errorf("%s panicked: %v", operation, r)
		}()

		return applyFn(ctx)
	}
}

func startBatchSpans(batch []*pendingWrite, name string, attrs ...attribute.KeyValue) []trace.Span {
	spans := make([]trace.Span, len(batch))
	for i, w := range batch {
		_, spans[i] = tracer.Start(w.ctx, name, trace.WithSpanKind(trace.SpanKindInternal), trace.WithAttributes(attrs...))
	}

	return spans
}

func endBatchSpans(spans []trace.Span, err error) {
	for _, s := range spans {
		endSpan(s, err)
	}
}

func (db *Database) commitWrites(batch []*pendingWrite) []*pendingWrite {
	db.proposeMu.Lock()
	defer db.proposeMu.Unlock()

	for _, w := range batch {
		w.waitSpan.End()
	}

	barrierSpans := startBatchSpans(batch, "db/write_barrier")
	err := db.writeBarrier()
	endBatchSpans(barrierSpans, err)

	if err != nil {
		for _, w := range batch {
			w.finish(nil, err)
		}

		return nil
	}

	if len(batch) == 1 || !db.appliedSchemaAtLeast(context.Background(), changesetBatchMinSchema) {
		for _, w := range batch {
			w.finish(db.captureAndProposeOne(w.ctx, w.operation, w.minSchema, w.applyFn))
		}

		return nil
	}

	captured, capturedSchema, carry, err := db.captureBatch(batch)
	if err != nil {
		for _, w := range batch {
			w.finish(nil, fmt.Errorf("capture changeset batch: %w", err))
		}

		return nil
	}

	db.proposeBatch(captured, capturedSchema)

	for _, w := range carry {
		_, w.waitSpan = tracer.Start(w.ctx, "db/propose_lock_wait", trace.WithSpanKind(trace.SpanKindInternal))
	}

	return carry
}

func (db *Database) captureAndProposeOne(ctx context.Context, operation string, minSchema int, applyFn func(context.Context) (any, error)) (*ellaraft.ProposeResult, error) {
	captureCtx, captureSpan := tracer.Start(ctx, "db/capture_changeset", trace.WithSpanKind(trace.SpanKindInternal))
	changeset, applyResult, capturedSchema, err := db.captureChangeset(context.WithoutCancel(captureCtx), recoverApply(operation, applyFn), operation)
	endSpan(captureSpan, err)

	if err != nil {
		return nil, wrapCaptureErr(operation, err)
	}

	if len(changeset) == 0 {
		return &ellaraft.ProposeResult{Value: applyResult}, nil
	}

	changesetCmd, err := ellaraft.NewCommand(ellaraft.CmdChangeset, &bytesPayload{
		Value:          changeset,
		Operation:      operation,
		RequiredSchema: minSchema,
		CapturedSchema: capturedSchema,
	})
	if err != nil {
		return nil, err
	}

	data, err := changesetCmd.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal changeset command: %w", err)
	}

	res, err := db.raftApply(ctx, data)
	if err != nil {
		return nil, err
	}

	logger.From(ctx, logger.DBLog).Debug("proposed changeset",
		zap.String("operation", operation),
		zap.Int("required_schema", minSchema),
		zap.Uint64("index", res.Index),
		logger.Bytes(uint64(len(changeset))))

	return &ellaraft.ProposeResult{Index: res.Index, Value: applyResult}, nil
}

func wrapCaptureErr(operation string, err error) error {
	if errors.Is(err, ErrAlreadyExists) ||
		errors.Is(err, ErrNotFound) ||
		errors.Is(err, ErrJoinTokenAlreadyConsumed) {
		return err
	}

	return fmt.Errorf("capture changeset for %s: %w", operation, err)
}

func encodedEntrySize(operation string, changeset []byte) int {
	return base64.StdEncoding.EncodedLen(len(changeset)) + len(operation) + groupCommitEntryFraming
}

func (db *Database) captureBatch(batch []*pendingWrite) ([]capturedWrite, int, []*pendingWrite, error) {
	ctx := context.Background()

	conn, err := db.conn().PlainDB().Conn(ctx)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("acquire sqlite conn for capture: %w", err)
	}

	defer func() { _ = conn.Close() }()

	captured := make([]capturedWrite, 0, len(batch))

	var (
		schema int
		carry  []*pendingWrite
	)

	err = conn.Raw(func(raw any) error {
		sqliteConn, err := rawSQLiteConn(raw)
		if err != nil {
			return err
		}

		dconn, ok := raw.(driver.Conn)
		if !ok {
			return fmt.Errorf("raw sqlite conn does not implement driver.Conn")
		}

		if _, err := sqliteConn.ExecContext(ctx, "BEGIN IMMEDIATE", nil); err != nil {
			return fmt.Errorf("begin changeset capture transaction: %w", err)
		}

		defer func() {
			_, _ = sqliteConn.ExecContext(context.Background(), "ROLLBACK", nil)
		}()

		schema, err = readSchemaVersionRaw(ctx, sqliteConn)
		if err != nil {
			return err
		}

		exec := func(stmt string) error {
			_, err := sqliteConn.ExecContext(ctx, stmt, nil)
			return err
		}

		start := time.Now()
		entryBytes := 0

		for i, w := range batch {
			if i > 0 && time.Since(start) > groupCommitCaptureBudget {
				carry = batch[i:]

				return nil
			}

			if err := exec("SAVEPOINT group_commit_write"); err != nil {
				if len(captured) == 0 {
					return fmt.Errorf("savepoint for %s: %w", w.operation, err)
				}

				carry = batch[i:]

				return nil
			}

			captureCtx, captureSpan := tracer.Start(w.ctx, "db/capture_changeset", trace.WithSpanKind(trace.SpanKindInternal))

			var value any

			applyFn := recoverApply(w.operation, w.applyFn)

			changeset, captureErr := sqliteConn.CaptureChangeset(context.WithoutCancel(captureCtx), func() error {
				v, applyErr := db.applyWithPinnedConn(context.WithoutCancel(captureCtx), dconn, applyFn)
				if applyErr != nil {
					return applyErr
				}

				value = v

				return nil
			}, replicatedChangesetTables)
			if captureErr != nil {
				captureErr = fmt.Errorf("capture sqlite changeset: %w", captureErr)
			}

			endSpan(captureSpan, captureErr)

			size := 0
			if len(changeset) > 0 {
				size = encodedEntrySize(w.operation, changeset)
			}

			overflow := captureErr == nil && entryBytes > 0 && entryBytes+size > groupCommitMaxEntryBytes
			dependsOnBatch := entryBytes > 0

			if captureErr != nil || overflow {
				if err := exec("ROLLBACK TO group_commit_write"); err != nil {
					if overflow {
						carry = batch[i:]

						return nil
					}

					captured = append(captured, capturedWrite{write: w, err: captureErr, dependsOnBatch: dependsOnBatch})
					carry = batch[i+1:]

					return nil
				}
			}

			if err := exec("RELEASE group_commit_write"); err != nil {
				releaseErr := fmt.Errorf("release savepoint for %s: %w", w.operation, err)
				if captureErr != nil {
					releaseErr = captureErr
				}

				if overflow {
					carry = batch[i:]

					return nil
				}

				captured = append(captured, capturedWrite{write: w, err: releaseErr, dependsOnBatch: dependsOnBatch})
				carry = batch[i+1:]

				return nil
			}

			if overflow {
				carry = batch[i:]

				return nil
			}

			if captureErr != nil {
				captured = append(captured, capturedWrite{write: w, err: captureErr, dependsOnBatch: dependsOnBatch})

				continue
			}

			entryBytes += size

			captured = append(captured, capturedWrite{write: w, changeset: changeset, value: value, dependsOnBatch: dependsOnBatch})
		}

		return nil
	})
	if err != nil {
		return nil, 0, nil, err
	}

	return captured, schema, carry, nil
}

func (c capturedWrite) ownOutcome(index uint64) (*ellaraft.ProposeResult, error) {
	if c.err != nil {
		return nil, wrapCaptureErr(c.write.operation, c.err)
	}

	return &ellaraft.ProposeResult{Index: index, Value: c.value}, nil
}

func (db *Database) proposeBatch(captured []capturedWrite, capturedSchema int) {
	var (
		entries  []changesetBatchEntry
		proposed []capturedWrite
		held     []capturedWrite
	)

	for _, c := range captured {
		switch {
		case len(c.changeset) > 0:
			entries = append(entries, changesetBatchEntry{
				Value:          c.changeset,
				Operation:      c.write.operation,
				RequiredSchema: c.write.minSchema,
			})
			proposed = append(proposed, c)
		case c.dependsOnBatch:
			held = append(held, c)
		default:
			c.write.finish(c.ownOutcome(0))
		}
	}

	if len(proposed) == 0 {
		for _, c := range held {
			c.write.finish(c.ownOutcome(0))
		}

		return
	}

	index, err := db.proposeEntries(entries, proposed, capturedSchema)
	if err != nil {
		for _, c := range proposed {
			c.write.finish(nil, err)
		}

		for _, c := range held {
			c.write.finish(nil, err)
		}

		return
	}

	for _, c := range proposed {
		c.write.finish(&ellaraft.ProposeResult{Index: index, Value: c.value}, nil)
	}

	for _, c := range held {
		c.write.finish(c.ownOutcome(index))
	}
}

func (db *Database) proposeEntries(entries []changesetBatchEntry, proposed []capturedWrite, capturedSchema int) (uint64, error) {
	var (
		cmd *ellaraft.Command
		err error
	)

	if len(entries) == 1 {
		cmd, err = ellaraft.NewCommand(ellaraft.CmdChangeset, &bytesPayload{
			Value:          entries[0].Value,
			Operation:      entries[0].Operation,
			RequiredSchema: entries[0].RequiredSchema,
			CapturedSchema: capturedSchema,
		})
	} else {
		cmd, err = ellaraft.NewCommand(ellaraft.CmdChangesetBatch, &changesetBatchPayload{
			Entries:        entries,
			CapturedSchema: capturedSchema,
		})
	}

	if err != nil {
		return 0, err
	}

	data, err := cmd.MarshalBinary()
	if err != nil {
		return 0, fmt.Errorf("marshal changeset command: %w", err)
	}

	writes := make([]*pendingWrite, len(proposed))
	for i, c := range proposed {
		writes[i] = c.write
	}

	spans := startBatchSpans(writes, "db/raft_apply", attribute.Int("ella.raft.batch_size", len(entries)))

	res, err := db.raftManager.ApplyBytes(data, db.proposeTimeout)
	if err != nil {
		err = classifyProposeErr(err)
		endBatchSpans(spans, err)

		return 0, err
	}

	for _, s := range spans {
		s.SetAttributes(attribute.Int64("ella.raft.index", int64(res.Index)))
	}

	endBatchSpans(spans, nil)

	logger.DBLog.Debug("proposed changeset batch",
		zap.Int("writes", len(entries)),
		zap.Uint64("index", res.Index),
		logger.Bytes(uint64(len(data))))

	return res.Index, nil
}
