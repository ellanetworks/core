// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"strings"
	"testing"
)

func TestAssertCapturedSchema(t *testing.T) {
	database := newStandaloneDB(t)
	ctx := t.Context()

	applied, err := database.CurrentSchemaVersion(ctx)
	if err != nil {
		t.Fatalf("CurrentSchemaVersion: %v", err)
	}

	if applied == 0 {
		t.Fatal("applied schema is 0")
	}

	t.Run("absent field is accepted", func(t *testing.T) {
		if err := database.assertCapturedSchema(ctx, 0, "changeset \"X\""); err != nil {
			t.Fatalf("captured 0 rejected: %v", err)
		}
	})

	t.Run("equal is accepted", func(t *testing.T) {
		if err := database.assertCapturedSchema(ctx, applied, "changeset \"X\""); err != nil {
			t.Fatalf("captured %d rejected: %v", applied, err)
		}
	})

	t.Run("behind is rejected", func(t *testing.T) {
		err := database.assertCapturedSchema(ctx, applied-1, "changeset \"X\"")
		if err == nil {
			t.Fatal("captured below applied was accepted")
		}

		if !strings.Contains(err.Error(), "captured at schema") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("ahead is rejected", func(t *testing.T) {
		if err := database.assertCapturedSchema(ctx, applied+1, "changeset \"X\""); err == nil {
			t.Fatal("captured above applied was accepted")
		}
	})

	t.Run("stale cache is re-read before rejecting", func(t *testing.T) {
		database.appliedSchemaCache.Store(int64(applied - 1))

		if err := database.assertCapturedSchema(ctx, applied, "changeset \"X\""); err != nil {
			t.Fatalf("stale cache caused a false rejection: %v", err)
		}

		if got := database.cachedAppliedSchema(); got != applied {
			t.Fatalf("cache not refreshed: got %d, want %d", got, applied)
		}
	})
}
