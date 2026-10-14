// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package apimatrix

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/internal/testenv"
)

// apiMatrixName prefixes resource names so they're identifiable as matrix
// state and so a collision from a leaked prior run surfaces immediately.
func apiMatrixName(resource string) string {
	return "apimat-" + resource
}

func assertNotFound(t *testing.T, err error, what string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %s lookup to fail with not-found, got nil", what)
	}

	msg := err.Error()
	assertf(t, strings.Contains(msg, "404") || strings.Contains(strings.ToLower(msg), "not found"), fmt.Sprintf("expected %s lookup to fail with not-found, got: %v", what, err))
}

func assertf(t *testing.T, condition bool, msg string) {
	t.Helper()

	if !condition {
		t.Fatalf("FAIL - %s", msg)
	}
}

func waitReady(ctx context.Context, c *client.Client) error {
	deadline := time.Now().Add(testenv.Timeout(2 * time.Minute))

	for time.Now().Before(deadline) {
		if status, err := c.GetStatus(ctx); err == nil && status.Ready {
			return nil
		}

		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for ella core to be ready")
}

func leaderAppliedIndex(ctx context.Context, leader *client.Client) (uint64, error) {
	status, err := leader.GetStatus(ctx)
	if err != nil {
		return 0, fmt.Errorf("get leader status: %w", err)
	}

	if !status.Cluster.Enabled {
		return 0, fmt.Errorf("leader has no cluster status")
	}

	return status.Cluster.AppliedIndex, nil
}

func waitForFollowerConvergence(ctx context.Context, clients []*client.Client, minIndex uint64) error {
	deadline := time.Now().Add(testenv.Timeout(30 * time.Second))

	for time.Now().Before(deadline) {
		if followersConverged(ctx, clients, minIndex) {
			return nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("followers did not reach applied index %d within 30s", minIndex)
}

func followersConverged(ctx context.Context, clients []*client.Client, minIndex uint64) bool {
	for _, c := range clients {
		status, err := c.GetStatus(ctx)
		if err != nil || !status.Cluster.Enabled {
			return false
		}

		if status.Cluster.Role == "Leader" {
			continue
		}

		if status.Cluster.Role != "Follower" || status.Cluster.AppliedIndex < minIndex || !status.Ready {
			return false
		}
	}

	return true
}

func findLeader(ctx context.Context, clients []*client.Client) (int, *client.Client, error) {
	timeout := testenv.Timeout(30 * time.Second)

	deadline := time.Now().Add(timeout)

	var lastState string

	for {
		leaderIdxs := make([]int, 0, len(clients))
		claims := make([]string, 0, len(clients))

		for i, c := range clients {
			status, err := c.GetStatus(ctx)
			if err != nil {
				continue
			}

			if status.Cluster.Enabled && status.Cluster.Role == "Leader" {
				leaderIdxs = append(leaderIdxs, i)
				claims = append(claims, fmt.Sprintf("node %s", status.Cluster.NodeID))
			}
		}

		if len(leaderIdxs) == 1 {
			return leaderIdxs[0], clients[leaderIdxs[0]], nil
		}

		if len(leaderIdxs) == 0 {
			lastState = "no node reports the Leader role"
		} else {
			lastState = fmt.Sprintf("%d nodes claim leadership at once (%s)",
				len(leaderIdxs), strings.Join(claims, ", "))
		}

		if !time.Now().Before(deadline) {
			return -1, nil, fmt.Errorf("no single leader after %v: %s", timeout, lastState)
		}

		select {
		case <-ctx.Done():
			return -1, nil, fmt.Errorf("waiting for a single leader: %w", ctx.Err())
		case <-time.After(time.Second):
		}
	}
}
