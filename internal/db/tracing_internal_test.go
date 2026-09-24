// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/otel/attribute"
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
			errorType: "*errors.errorString",
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
