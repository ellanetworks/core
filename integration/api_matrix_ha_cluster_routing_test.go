// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ellanetworks/core/client"
)

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

func runClusterRoutingHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	t.Run("promote_on_a_follower_reaches_the_leader", func(t *testing.T) {
		follower, leader := leaderAndFollower(ctx, t, h)

		err := follower.PromoteClusterMember(ctx, leader.NodeID)
		if err == nil || !strings.Contains(err.Error(), "already a voter") {
			t.Fatalf("PromoteClusterMember on a follower: got %v, want the leader's already-a-voter conflict", err)
		}
	})

	t.Run("members_advertise_a_usable_api_address", func(t *testing.T) {
		follower, _ := leaderAndFollower(ctx, t, h)

		members, err := follower.ListClusterMembers(ctx)
		if err != nil {
			t.Fatalf("list cluster members: %v", err)
		}

		for _, m := range members {
			if m.APIAddress == "" || strings.Contains(m.APIAddress, "0.0.0.0") || strings.Contains(m.APIAddress, "[::]") {
				t.Errorf("member %s advertises API address %q", m.NodeID, m.APIAddress)
			}
		}
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
