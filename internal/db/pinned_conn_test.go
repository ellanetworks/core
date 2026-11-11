// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
)

type recordingConn struct {
	prepares int
	execs    int
	queries  int
}

func (c *recordingConn) Prepare(string) (driver.Stmt, error) {
	c.prepares++
	return nil, errors.New("prepare not expected")
}

func (c *recordingConn) Close() error { return nil }

func (c *recordingConn) Begin() (driver.Tx, error) {
	return nil, errors.New("begin not expected")
}

func (c *recordingConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	c.execs++
	return driver.RowsAffected(1), nil
}

func (c *recordingConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	c.queries++
	return emptyRows{}, nil
}

type emptyRows struct{}

func (emptyRows) Columns() []string         { return []string{"x"} }
func (emptyRows) Close() error              { return nil }
func (emptyRows) Next([]driver.Value) error { return io.EOF }

func TestPinnedConnUsesDirectExecAndQuery(t *testing.T) {
	inner := &recordingConn{}

	pinned := sql.OpenDB(&pinnedConnector{conn: inner})
	pinned.SetMaxOpenConns(1)

	t.Cleanup(func() { _ = pinned.Close() })

	ctx := context.Background()

	if _, err := pinned.ExecContext(ctx, "UPDATE t SET x=?", 1); err != nil {
		t.Fatalf("exec: %v", err)
	}

	var x int
	if err := pinned.QueryRowContext(ctx, "SELECT x FROM t WHERE x=?", 1).Scan(&x); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("query: expected ErrNoRows, got %v", err)
	}

	if inner.execs != 1 || inner.queries != 1 {
		t.Fatalf("expected 1 exec and 1 query on the inner conn, got %d and %d", inner.execs, inner.queries)
	}

	if inner.prepares != 0 {
		t.Fatalf("expected no Prepare fallback, got %d", inner.prepares)
	}
}
