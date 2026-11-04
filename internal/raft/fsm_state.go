// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/raft"
)

type AppliedState struct {
	Index              uint64
	Term               uint64
	Configuration      []byte
	ConfigurationIndex uint64
}

func (s AppliedState) snapshotMeta(id string) *raft.SnapshotMeta {
	meta := &raft.SnapshotMeta{
		Version:            raft.SnapshotVersionMax,
		ID:                 id,
		Index:              s.Index,
		Term:               s.Term,
		ConfigurationIndex: s.ConfigurationIndex,
	}

	if s.Configuration != nil {
		meta.Configuration = raft.DecodeConfiguration(s.Configuration)
	}

	return meta
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type sqlQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type sqlExecQueryer interface {
	sqlExecer
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func EnsureFSMStateSchema(ctx context.Context, db sqlExecQueryer) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS fsm_state (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			lastApplied INTEGER NOT NULL DEFAULT 0,
			lastAppliedTerm INTEGER NOT NULL DEFAULT 0,
			configuration BLOB,
			configurationIndex INTEGER NOT NULL DEFAULT 0,
			durableWrites INTEGER NOT NULL DEFAULT 0
		)`); err != nil {
		return fmt.Errorf("create fsm_state table: %w", err)
	}

	columns, err := fsmStateColumns(ctx, db)
	if err != nil {
		return err
	}

	for _, col := range []struct{ name, ddl string }{
		{"lastAppliedTerm", "ALTER TABLE fsm_state ADD COLUMN lastAppliedTerm INTEGER NOT NULL DEFAULT 0"},
		{"configuration", "ALTER TABLE fsm_state ADD COLUMN configuration BLOB"},
		{"configurationIndex", "ALTER TABLE fsm_state ADD COLUMN configurationIndex INTEGER NOT NULL DEFAULT 0"},
		{"durableWrites", "ALTER TABLE fsm_state ADD COLUMN durableWrites INTEGER NOT NULL DEFAULT 0"},
	} {
		if columns[col.name] {
			continue
		}

		if _, err := db.ExecContext(ctx, col.ddl); err != nil {
			return fmt.Errorf("add fsm_state.%s: %w", col.name, err)
		}
	}

	if _, err := db.ExecContext(ctx,
		"INSERT OR IGNORE INTO fsm_state (id, lastApplied) VALUES (1, 0)"); err != nil {
		return fmt.Errorf("seed fsm_state: %w", err)
	}

	return nil
}

func fsmStateColumns(ctx context.Context, db sqlExecQueryer) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, "SELECT name FROM pragma_table_info('fsm_state')")
	if err != nil {
		return nil, fmt.Errorf("list fsm_state columns: %w", err)
	}

	defer func() { _ = rows.Close() }()

	columns := map[string]bool{}

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan fsm_state column: %w", err)
		}

		columns[name] = true
	}

	return columns, rows.Err()
}

func readAppliedState(ctx context.Context, db sqlQueryer) (AppliedState, error) {
	var s AppliedState

	err := db.QueryRowContext(ctx,
		"SELECT lastApplied, lastAppliedTerm, configuration, configurationIndex FROM fsm_state WHERE id = 1",
	).Scan(&s.Index, &s.Term, &s.Configuration, &s.ConfigurationIndex)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no such table") {
			return AppliedState{}, nil
		}

		return AppliedState{}, fmt.Errorf("read fsm_state: %w", err)
	}

	return s, nil
}

func advanceAppliedPosition(ctx context.Context, db sqlExecer, index, term uint64) error {
	if _, err := db.ExecContext(ctx, `
		UPDATE fsm_state SET
			lastAppliedTerm = CASE WHEN lastApplied < ?1 THEN ?2 ELSE lastAppliedTerm END,
			lastApplied = MAX(lastApplied, ?1)
		WHERE id = 1`,
		index, term); err != nil {
		return fmt.Errorf("advance fsm_state.lastApplied to %d: %w", index, err)
	}

	return nil
}

func advanceConfiguration(ctx context.Context, db sqlExecer, configuration []byte, index uint64) error {
	if _, err := db.ExecContext(ctx, `
		UPDATE fsm_state SET
			configuration = CASE WHEN configuration IS NULL OR configurationIndex < ?1 THEN ?2 ELSE configuration END,
			configurationIndex = CASE WHEN configuration IS NULL OR configurationIndex < ?1 THEN ?1 ELSE configurationIndex END
		WHERE id = 1`,
		index, configuration); err != nil {
		return fmt.Errorf("advance fsm_state.configuration to index %d: %w", index, err)
	}

	return nil
}

func setConfiguration(ctx context.Context, db sqlExecer, configuration []byte, index uint64) error {
	if _, err := db.ExecContext(ctx,
		"UPDATE fsm_state SET configuration = ?, configurationIndex = ? WHERE id = 1",
		configuration, index); err != nil {
		return fmt.Errorf("write fsm_state.configuration at index %d: %w", index, err)
	}

	return nil
}

func advanceAppliedState(ctx context.Context, db sqlExecer, s AppliedState) error {
	if err := advanceAppliedPosition(ctx, db, s.Index, s.Term); err != nil {
		return err
	}

	if s.Configuration == nil {
		return nil
	}

	return advanceConfiguration(ctx, db, s.Configuration, s.ConfigurationIndex)
}

func overwriteAppliedState(ctx context.Context, db sqlExecer, s AppliedState) error {
	if _, err := db.ExecContext(ctx,
		"UPDATE fsm_state SET lastApplied = ?, lastAppliedTerm = ?, configuration = ?, configurationIndex = ? WHERE id = 1",
		s.Index, s.Term, s.Configuration, s.ConfigurationIndex); err != nil {
		return fmt.Errorf("write fsm_state: %w", err)
	}

	return nil
}

func durably(ctx context.Context, db *sql.DB, fn func(sqlExecer) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}

	defer func() { _ = conn.Close() }()

	var previous int
	if err := conn.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&previous); err != nil {
		return fmt.Errorf("read synchronous mode: %w", err)
	}

	if _, err := conn.ExecContext(ctx, "PRAGMA synchronous = FULL"); err != nil {
		return fmt.Errorf("enable synchronous writes: %w", err)
	}

	defer func() {
		_, _ = conn.ExecContext(context.Background(), fmt.Sprintf("PRAGMA synchronous = %d", previous))
	}()

	if err := fn(conn); err != nil {
		return err
	}

	if _, err := conn.ExecContext(ctx, "UPDATE fsm_state SET durableWrites = durableWrites + 1 WHERE id = 1"); err != nil {
		return fmt.Errorf("sync fsm_state: %w", err)
	}

	return nil
}
