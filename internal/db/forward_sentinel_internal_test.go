// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"errors"
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func TestSentinelForForwardCode_RoundTripsDomainErrors(t *testing.T) {
	for _, tc := range []struct {
		code string
		want error
	}{
		{ellaraft.ForwardCodeTokenConsumed, ErrJoinTokenAlreadyConsumed},
		{ellaraft.ForwardCodeTokenExpired, ErrJoinTokenExpired},
		{ellaraft.ForwardCodeTokenNodeMism, ErrJoinTokenNodeMismatch},
		{ellaraft.ForwardCodeMigrationPend, ErrMigrationPending},
		{ellaraft.ForwardCodeNotFound, ErrNotFound},
		{ellaraft.ForwardCodeAlreadyExists, ErrAlreadyExists},
	} {
		if got := sentinelForForwardCode(tc.code); !errors.Is(got, tc.want) {
			t.Errorf("code %q rehydrated as %v, want %v", tc.code, got, tc.want)
		}
	}

	if got := sentinelForForwardCode(""); got != nil {
		t.Errorf("uncoded error rehydrated as %v, want nil", got)
	}
}
