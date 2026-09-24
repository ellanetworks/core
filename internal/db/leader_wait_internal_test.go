// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestHoldForLeader(t *testing.T) {
	t.Run("without raft", func(t *testing.T) {
		if err := (&Database{}).holdForLeader(t.Context()); err != nil {
			t.Fatalf("holdForLeader = %v, want nil", err)
		}
	})

	t.Run("with a leader", func(t *testing.T) {
		if err := newStandaloneDB(t).holdForLeader(t.Context()); err != nil {
			t.Fatalf("holdForLeader = %v, want nil", err)
		}
	})

	t.Run("without a leader", func(t *testing.T) {
		database := newFollowerDatabase(t)

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		if err := database.holdForLeader(ctx); !errors.Is(err, ErrProposeTimeout) {
			t.Fatalf("holdForLeader = %v, want ErrProposeTimeout", err)
		}
	})
}
