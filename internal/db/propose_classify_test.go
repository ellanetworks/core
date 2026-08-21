// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"errors"
	"fmt"
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
	hraft "github.com/hashicorp/raft"
)

func TestClassifyBarrierErr_LeadershipLossIsNotOutcomeUnknown(t *testing.T) {
	for _, cause := range []error{hraft.ErrLeadershipLost, hraft.ErrNotLeader} {
		got := classifyBarrierErr(cause)

		if errors.Is(got, ErrOutcomeUnknown) {
			t.Fatalf("barrier runs before capture; %v must not be outcome-unknown, got %v", cause, got)
		}

		if !errors.Is(got, hraft.ErrNotLeader) {
			t.Fatalf("want ErrNotLeader so the forwarder retries, got %v", got)
		}
	}
}

func TestClassifyBarrierErr_TimeoutStaysRetryable(t *testing.T) {
	got := classifyBarrierErr(fmt.Errorf("wrapped: %w", ellaraft.ErrBarrierTimeout))
	if !errors.Is(got, ErrProposeTimeout) {
		t.Fatalf("want ErrProposeTimeout, got %v", got)
	}
}

func TestClassifyProposeErr_NotLeaderStaysForwardable(t *testing.T) {
	got := classifyProposeErr(hraft.ErrNotLeader)

	if !errors.Is(got, hraft.ErrNotLeader) {
		t.Fatalf("ErrNotLeader must survive classification so Invoke can forward, got %v", got)
	}

	if errors.Is(got, ErrProposeTimeout) {
		t.Fatal("ErrNotLeader must not be laundered into ErrProposeTimeout")
	}
}

func TestClassifyProposeErr_LeadershipLostIsOutcomeUnknown(t *testing.T) {
	got := classifyProposeErr(hraft.ErrLeadershipLost)

	if !errors.Is(got, ErrOutcomeUnknown) {
		t.Fatalf("want ErrOutcomeUnknown, got %v", got)
	}

	if errors.Is(got, hraft.ErrNotLeader) {
		t.Fatal("an ambiguous outcome must not be forwardable")
	}
}

func TestClassifyProposeErr_ShutdownStaysTransient(t *testing.T) {
	got := classifyProposeErr(hraft.ErrRaftShutdown)
	if !errors.Is(got, ErrProposeTimeout) {
		t.Fatalf("want ErrProposeTimeout, got %v", got)
	}
}
