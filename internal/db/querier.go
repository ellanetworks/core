// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"encoding/json"
)

type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type scanner interface {
	Scan(dest ...any) error
}

type countedRow struct {
	scanner
	total *int
}

func (r countedRow) Scan(dest ...any) error {
	return r.scanner.Scan(append(dest, r.total)...)
}

func queryAll[T any](ctx context.Context, q querier, query string, scan func(scanner) (T, error), args ...any) ([]T, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	defer func() { _ = rows.Close() }()

	var out []T

	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, v)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func queryPageWithTotal[T any](ctx context.Context, q querier, query string, scan func(scanner) (T, error), args ...any) ([]T, int, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}

	defer func() { _ = rows.Close() }()

	var (
		out   []T
		total int
	)

	for rows.Next() {
		v, err := scan(countedRow{scanner: rows, total: &total})
		if err != nil {
			return nil, 0, err
		}

		out = append(out, v)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return out, total, nil
}

func scanValue[T any](row scanner) (T, error) {
	var v T

	if err := row.Scan(&v); err != nil {
		var zero T

		return zero, err
	}

	return v, nil
}

func jsonArray(values []string) string {
	b, _ := json.Marshal(values)

	return string(b)
}

func queryCount(ctx context.Context, q querier, query string, args ...any) (int, error) {
	var n int

	err := q.QueryRowContext(ctx, query, args...).Scan(&n)

	return n, err
}
