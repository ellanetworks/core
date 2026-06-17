// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRunJoinFlow_EmptyTokenIsNoOp(t *testing.T) {
	if err := runJoinFlow(context.Background(), nil, []string{"127.0.0.1:1"}, ""); err != nil {
		t.Fatalf("empty token should be a no-op, got %v", err)
	}
}

func TestRunJoinFlow_EmptyPeersWithTokenErrors(t *testing.T) {
	err := runJoinFlow(context.Background(), nil, nil, "tok")
	if err == nil {
		t.Fatal("expected error for empty peers with non-empty token")
	}

	if !strings.Contains(err.Error(), "cluster.peers is empty") {
		t.Fatalf("expected peers-empty error, got %v", err)
	}
}

// TestRunJoinFlow_RetriesUntilContextCancelled drives runJoinFlow
// against an unreachable address. JoinFlow returns before the first
// dial (parse-token failure on the malformed token), so each pass
// completes in microseconds; cancelling ctx mid-loop must make the
// function return promptly with a joined error that includes ctx.Err.
func TestRunJoinFlow_RetriesUntilContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)

	go func() {
		done <- runJoinFlow(ctx, nil, []string{"127.0.0.1:1"}, "not-a-real-token")
	}()

	// Let the loop accumulate at least one pass before cancelling.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected non-nil error after cancel")
		}

		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected wrapped context.Canceled, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runJoinFlow did not return within 5s of ctx cancel")
	}
}
