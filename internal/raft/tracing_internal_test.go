// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"errors"
	"fmt"
	"testing"

	hraft "github.com/hashicorp/raft"
)

func TestSpanErrorType(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"forward code from leader", fmt.Errorf("forward operation: %w", &ForwardCodedError{Code: ForwardCodeAlreadyExists, Message: "exists"}), "already_exists"},
		{"leader request not sent", fmt.Errorf("%w: new request: %w", ErrLeaderRequestNotSent, errors.New("bad url")), "leader_request_not_sent"},
		{"leader unreachable", fmt.Errorf("%w: dial: %w", ErrLeaderUnreachable, errors.New("refused")), "leader_unreachable"},
		{"outcome unknown", fmt.Errorf("forward: %w", ErrOutcomeUnknown), "outcome_unknown"},
		{"not leader", hraft.ErrNotLeader, "not_leader"},
		{"deadline", fmt.Errorf("forward: %w", context.DeadlineExceeded), "timeout"},
		{"canceled", context.Canceled, "canceled"},
		{"unknown", errors.New("boom"), "*errors.errorString"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := spanErrorType(tc.err).Value.AsString(); got != tc.want {
				t.Fatalf("error.type = %q, want %q", got, tc.want)
			}
		})
	}
}
