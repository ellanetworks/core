// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"net/url"
	"testing"
	"time"
)

func TestParseTimeParam(t *testing.T) {
	def := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		raw     string
		want    time.Time
		wantErr bool
	}{
		{"absent returns default", "", def, false},
		{"blank returns default", "   ", def, false},
		{"rfc3339 utc", "2026-09-15T14:30:00Z", time.Date(2026, 9, 15, 14, 30, 0, 0, time.UTC), false},
		{"rfc3339 offset", "2026-09-15T10:30:00-04:00", time.Date(2026, 9, 15, 14, 30, 0, 0, time.UTC), false},
		{"rfc3339 nano", "2026-09-15T14:30:00.123456789Z", time.Date(2026, 9, 15, 14, 30, 0, 123456789, time.UTC), false},
		{"calendar date rejected", "2026-09-15", time.Time{}, true},
		{"unix epoch rejected", "1789476600", time.Time{}, true},
		{"garbage rejected", "not-a-timestamp", time.Time{}, true},
		{"missing offset rejected", "2026-09-15T14:30:00", time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := url.Values{}
			q.Set("start", tt.raw)

			got, err := parseTimeParam(q, "start", def)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %v", got)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if !got.Equal(tt.want) {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestParseTimeParamNamesTheParameter(t *testing.T) {
	q := url.Values{}
	q.Set("end", "nonsense")

	_, err := parseTimeParam(q, "end", time.Time{})
	if err == nil {
		t.Fatal("expected an error")
	}

	if got := err.Error(); got[:len("invalid end")] != "invalid end" {
		t.Fatalf("expected the error to name the parameter, got %q", got)
	}
}

func TestParseTimeRange(t *testing.T) {
	defStart := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	defEnd := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("both absent fall back to defaults", func(t *testing.T) {
		start, end, err := parseTimeRange(url.Values{}, defStart, defEnd)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if !start.Equal(defStart) || !end.Equal(defEnd) {
			t.Fatalf("expected defaults, got %s / %s", start, end)
		}
	})

	t.Run("bounds are independent", func(t *testing.T) {
		q := url.Values{}
		q.Set("start", "2026-09-15T14:30:00Z")

		start, end, err := parseTimeRange(q, time.Time{}, time.Time{})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if start.IsZero() {
			t.Fatal("expected start to be set")
		}

		if !end.IsZero() {
			t.Fatalf("expected end to stay unbounded, got %s", end)
		}
	})

	t.Run("inverted range is rejected", func(t *testing.T) {
		q := url.Values{}
		q.Set("start", "2026-09-15T14:30:00Z")
		q.Set("end", "2026-09-15T14:29:59Z")

		if _, _, err := parseTimeRange(q, time.Time{}, time.Time{}); err == nil {
			t.Fatal("expected an error")
		}
	})

	t.Run("equal bounds are accepted", func(t *testing.T) {
		q := url.Values{}
		q.Set("start", "2026-09-15T14:30:00Z")
		q.Set("end", "2026-09-15T14:30:00Z")

		if _, _, err := parseTimeRange(q, time.Time{}, time.Time{}); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	})

	t.Run("an unbounded side never inverts", func(t *testing.T) {
		q := url.Values{}
		q.Set("end", "2026-09-15T14:30:00Z")

		if _, _, err := parseTimeRange(q, time.Time{}, time.Time{}); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	})
}
