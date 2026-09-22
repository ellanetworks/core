// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

const notLeaderAttempts = 3

func leaderAndFollower(ctx context.Context, t *testing.T, h *haMatrixEnv) (*client.Client, client.ClusterMember) {
	t.Helper()

	leaderIdx, leader, err := findLeader(ctx, h.Clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	members, err := leader.ListClusterMembers(ctx)
	if err != nil {
		t.Fatalf("list cluster members: %v", err)
	}

	for _, m := range members {
		if m.IsLeader {
			return h.Clients[(leaderIdx+1)%len(h.Clients)], m
		}
	}

	t.Fatal("no member reports isLeader")

	return nil, client.ClusterMember{}
}

func assertNamesTheLeader(ctx context.Context, t *testing.T, h *haMatrixEnv, call func(*client.Client) error) {
	t.Helper()

	for attempt := range notLeaderAttempts {
		follower, leader := leaderAndFollower(ctx, t, h)

		err := call(follower)
		if err == nil || !strings.Contains(err.Error(), "not the cluster leader") {
			t.Logf("attempt %d: got %v, want a not-leader rejection; leadership may have moved, retrying", attempt+1, err)
			continue
		}

		if !strings.Contains(err.Error(), string(leader.NodeID)) {
			t.Fatalf("rejection %q does not name the leader %s", err, leader.NodeID)
		}

		if !strings.Contains(err.Error(), leader.APIAddress) {
			t.Fatalf("rejection %q does not name the leader's API address %s", err, leader.APIAddress)
		}

		return
	}

	t.Fatalf("no not-leader rejection after %d attempts", notLeaderAttempts)
}

func runClusterRoutingHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	t.Run("promote_names_the_leader", func(t *testing.T) {
		assertNamesTheLeader(ctx, t, h, func(follower *client.Client) error {
			status, err := follower.GetStatus(ctx)
			if err != nil {
				return fmt.Errorf("get status: %w", err)
			}

			return follower.PromoteClusterMember(ctx, status.Cluster.NodeID)
		})
	})

	t.Run("mint_join_token_works_on_a_follower", func(t *testing.T) {
		follower, _ := leaderAndFollower(ctx, t, h)

		if _, err := follower.MintClusterJoinToken(ctx, &client.MintJoinTokenOptions{TTLSeconds: 600}); err != nil {
			t.Fatalf("MintClusterJoinToken on a follower: %v", err)
		}
	})

	t.Run("autopilot_reads_proxy_to_the_leader", func(t *testing.T) {
		follower, _ := leaderAndFollower(ctx, t, h)

		state, err := follower.GetAutopilotState(ctx)
		if err != nil {
			t.Fatalf("GetAutopilotState on a follower: %v", err)
		}

		if !state.Healthy {
			t.Errorf("autopilot reports unhealthy: %+v", state)
		}

		if state.FailureTolerance != 1 {
			t.Errorf("failureTolerance = %d, want 1", state.FailureTolerance)
		}

		if len(state.Servers) != len(h.Clients) {
			t.Errorf("servers = %d, want %d", len(state.Servers), len(h.Clients))
		}
	})
}
