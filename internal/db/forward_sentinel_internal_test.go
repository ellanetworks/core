// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"errors"
	"fmt"
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func TestForwardCodes_RoundTripDomainErrors(t *testing.T) {
	for _, tc := range []struct {
		code     string
		sentinel error
	}{
		{ellaraft.ForwardCodeTokenConsumed, ErrJoinTokenAlreadyConsumed},
		{ellaraft.ForwardCodeTokenExpired, ErrJoinTokenExpired},
		{ellaraft.ForwardCodeTokenNodeMism, ErrJoinTokenNodeMismatch},
		{ellaraft.ForwardCodeMigrationPend, ErrMigrationPending},
		{ellaraft.ForwardCodeNotFound, ErrNotFound},
		{ellaraft.ForwardCodeAlreadyExists, ErrAlreadyExists},
	} {
		if got := sentinelForForwardCode(tc.code); !errors.Is(got, tc.sentinel) {
			t.Errorf("sentinelForForwardCode(%q) = %v, want %v", tc.code, got, tc.sentinel)
		}

		if got := ForwardCodeFor(fmt.Errorf("wrapped: %w", tc.sentinel)); got != tc.code {
			t.Errorf("ForwardCodeFor(%v) = %q, want %q", tc.sentinel, got, tc.code)
		}
	}

	if got := sentinelForForwardCode(""); got != nil {
		t.Errorf("sentinelForForwardCode(\"\") = %v, want nil", got)
	}

	if got := ForwardCodeFor(errors.New("unrelated")); got != "" {
		t.Errorf("ForwardCodeFor(unrelated) = %q, want empty", got)
	}
}
