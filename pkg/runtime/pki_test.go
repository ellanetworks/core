// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/joinreq"
	"github.com/ellanetworks/core/internal/cluster/pkiagent"
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

func TestApplyJoinRequest_RefusesWhenALeafIsAlreadyOnDisk(t *testing.T) {
	agent := pkiagent.NewAgent("01a0c4a0-b76e-7050-ab95-218317ecef57", "cluster-a", t.TempDir())
	if err := agent.GenerateAndPersist(); err != nil {
		t.Fatalf("seed a leaf on disk: %v", err)
	}

	if !agent.HaveLeafOnDisk() {
		t.Fatal("test setup: expected a leaf on disk")
	}

	err := applyJoinRequest(context.Background(), &pkiState{agent: agent}, nil, joinreq.Request{
		Mode:          joinreq.ModeJoin,
		Token:         strings.Repeat("A", 96),
		SeedAddresses: []string{"10.0.0.1:7000"},
	})
	if err == nil {
		t.Fatal("a node holding a cluster certificate must not accept a join")
	}

	if !strings.Contains(err.Error(), "delete its data directory") {
		t.Fatalf("error must tell the operator to clear the node, got %v", err)
	}
}
