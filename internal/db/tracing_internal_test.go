// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mattn/go-sqlite3"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

func attrValue(attrs []attribute.KeyValue, key attribute.Key) (string, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value.AsString(), true
		}
	}

	return "", false
}

func TestSpanErrorAttributes(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		errorType  string
		statusCode string
	}{
		{
			name:       "sqlite constraint",
			err:        fmt.Errorf("query failed: %w", sqlite3.Error{Code: sqlite3.ErrConstraint, ExtendedCode: sqlite3.ErrConstraintUnique}),
			errorType:  "2067",
			statusCode: "2067",
		},
		{
			name:      "wrapped sentinel",
			err:       fmt.Errorf("get subscriber: %w", ErrNotFound),
			errorType: "not_found",
		},
		{
			name:      "context deadline",
			err:       fmt.Errorf("propose: %w", context.DeadlineExceeded),
			errorType: "timeout",
		},
		{
			name:      "unknown error",
			err:       errors.New("boom"),
			errorType: "_OTHER",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			attrs := spanErrorAttributes(tc.err)

			got, ok := attrValue(attrs, semconv.ErrorTypeKey)
			if !ok || got != tc.errorType {
				t.Fatalf("error.type = %q (present=%v), want %q", got, ok, tc.errorType)
			}

			code, ok := attrValue(attrs, semconv.DBResponseStatusCodeKey)
			if tc.statusCode == "" {
				if ok {
					t.Fatalf("db.response.status_code = %q, want unset", code)
				}

				return
			}

			if code != tc.statusCode {
				t.Fatalf("db.response.status_code = %q, want %q", code, tc.statusCode)
			}
		})
	}
}

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()

	var m dto.Metric
	if err := c.Write(&m); err != nil {
		t.Fatalf("read counter: %v", err)
	}

	return m.GetCounter().GetValue()
}

func histogramCount(t *testing.T, o prometheus.Observer) uint64 {
	t.Helper()

	h, ok := o.(prometheus.Metric)
	if !ok {
		t.Fatalf("observer %T is not a metric", o)
	}

	var m dto.Metric
	if err := h.Write(&m); err != nil {
		t.Fatalf("read histogram: %v", err)
	}

	return m.GetHistogram().GetSampleCount()
}

func TestFinishRecordsOutcomeOnSpanAndMetrics(t *testing.T) {
	if _, err := NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), DBFilename)); err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	exporter := recordTestSpans(t)

	tests := []struct {
		name      string
		err       error
		errorType string
	}{
		{name: "success"},
		{name: "not found", err: fmt.Errorf("get: %w", ErrNotFound)},
		{name: "no rows", err: fmt.Errorf("query failed: %w", sql.ErrNoRows)},
		{name: "already exists", err: ErrAlreadyExists},
		{name: "busy", err: fmt.Errorf("query failed: %w", sqlite3.Error{Code: sqlite3.ErrBusy, ExtendedCode: sqlite3.ErrNoExtended(sqlite3.ErrBusy)}), errorType: "5"},
		{name: "unique constraint", err: sqlite3.Error{Code: sqlite3.ErrConstraint, ExtendedCode: sqlite3.ErrConstraintUnique}, errorType: "2067"},
		{name: "canceled", err: fmt.Errorf("query failed: %w", context.Canceled)},
		{name: "propose timeout", err: fmt.Errorf("write: %w", ErrProposeTimeout), errorType: "propose_timeout"},
		{name: "unclassified", err: errors.New("boom"), errorType: "_OTHER"},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			table := "finish_test_" + strconv.Itoa(i)

			before := counterValue(t, DBQueriesTotal.WithLabelValues(table, "batch_insert", tc.errorType))

			_, finish := startDBQuery(context.Background(), table, "BATCH INSERT")
			finish(tc.err)

			if got := counterValue(t, DBQueriesTotal.WithLabelValues(table, "batch_insert", tc.errorType)) - before; got != 1 {
				t.Fatalf("app_database_queries_total{error_type=%q} increased by %v, want 1", tc.errorType, got)
			}

			if n := histogramCount(t, DBQueryDuration.WithLabelValues(table, "batch_insert")); n != 1 {
				t.Fatalf("app_database_query_duration_seconds count for %s = %d, want 1", table, n)
			}

			span := requireSpan(t, exporter.GetSpans(), "BATCH INSERT "+table)

			got, set := attrValue(span.Attributes, semconv.ErrorTypeKey)
			if tc.errorType == "" {
				if set || span.Status.Code != codes.Unset {
					t.Fatalf("span error.type = %q, status = %v; want unset", got, span.Status.Code)
				}

				return
			}

			if got != tc.errorType || span.Status.Code != codes.Error {
				t.Fatalf("span error.type = %q, status = %v; want %q, Error", got, span.Status.Code, tc.errorType)
			}
		})
	}
}

var dbOperationNames = map[string]bool{
	"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true, "UPSERT": true,
	"REPLACE": true, "ROTATE": true, "BATCH INSERT": true,
}

func TestDBQueryOperationsUseTheFixedLabelSet(t *testing.T) {
	fset := token.NewFileSet()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	checked := 0

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			var opArg int

			switch callName(call.Fun) {
			case "startDBQuery":
				opArg = 2
			case "startDBQueryNamed":
				opArg = 3
			default:
				return true
			}

			lit, ok := call.Args[opArg].(*ast.BasicLit)
			if !ok {
				return true
			}

			op, _ := strconv.Unquote(lit.Value)
			if !dbOperationNames[op] {
				t.Errorf("%s: operation %q is not in the fixed label set", fset.Position(call.Pos()), op)
			}

			checked++

			return true
		})
	}

	if checked < 200 {
		t.Fatalf("checked only %d call sites; the matcher no longer finds startDBQuery calls", checked)
	}
}
