// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	hraft "github.com/hashicorp/raft"
)

func TestIsUnavailable(t *testing.T) {
	cases := map[error]bool{
		fmt.Errorf("cas: %w", db.ErrProposeTimeout):   true,
		fmt.Errorf("cas: %w", db.ErrOutcomeUnknown):   true,
		fmt.Errorf("cas: %w", db.ErrMigrationPending): true,
		fmt.Errorf("cas: %w", hraft.ErrNotLeader):     true,
		hraft.ErrLeadershipLost:                       true,
		db.ErrNotFound:                                false,
		errors.New("disk I/O error"):                  false,
		nil:                                           false,
	}

	for err, want := range cases {
		if got := db.IsUnavailable(err); got != want {
			t.Errorf("IsUnavailable(%v) = %v, want %v", err, got, want)
		}
	}
}
