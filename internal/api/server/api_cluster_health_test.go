// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"testing"

	"github.com/ellanetworks/core/internal/pki"
)

func TestSummarizeClusterHealth(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state AutopilotStateResponse
		want  ClusterHealthResponse
	}{
		{
			name: "all voters healthy",
			state: AutopilotStateResponse{
				Healthy:          true,
				FailureTolerance: 1,
				LeaderNodeID:     "a",
				Voters:           []pki.NodeID{"a", "b", "c"},
				Servers: []AutopilotServerResponse{
					{NodeID: "a", Healthy: true, HasVotingRights: true},
					{NodeID: "b", Healthy: true, HasVotingRights: true},
					{NodeID: "c", Healthy: true, HasVotingRights: true},
				},
			},
			want: ClusterHealthResponse{
				Enabled: true, HasLeader: true, Healthy: true,
				HealthyVoters: 3, TotalVoters: 3, FailureTolerance: 1,
			},
		},
		{
			name: "one voter down",
			state: AutopilotStateResponse{
				Healthy:          false,
				FailureTolerance: 0,
				LeaderNodeID:     "a",
				Voters:           []pki.NodeID{"a", "b", "c"},
				Servers: []AutopilotServerResponse{
					{NodeID: "a", Healthy: true, HasVotingRights: true},
					{NodeID: "b", Healthy: true, HasVotingRights: true},
					{NodeID: "c", Healthy: false, HasVotingRights: true},
				},
			},
			want: ClusterHealthResponse{
				Enabled: true, HasLeader: true, Healthy: false,
				HealthyVoters: 2, TotalVoters: 3, FailureTolerance: 0,
			},
		},
		{
			name: "nonvoters stay out of the ratio",
			state: AutopilotStateResponse{
				Healthy:          true,
				FailureTolerance: 0,
				LeaderNodeID:     "a",
				Voters:           []pki.NodeID{"a"},
				Servers: []AutopilotServerResponse{
					{NodeID: "a", Healthy: true, HasVotingRights: true},
					{NodeID: "b", Healthy: true, HasVotingRights: false},
					{NodeID: "c", Healthy: false, HasVotingRights: false},
				},
			},
			want: ClusterHealthResponse{
				Enabled: true, HasLeader: true, Healthy: true,
				HealthyVoters: 1, TotalVoters: 1, FailureTolerance: 0,
			},
		},
		{
			name: "voter with no reported server",
			state: AutopilotStateResponse{
				Healthy:      false,
				LeaderNodeID: "a",
				Voters:       []pki.NodeID{"a", "b"},
				Servers: []AutopilotServerResponse{
					{NodeID: "a", Healthy: true, HasVotingRights: true},
				},
			},
			want: ClusterHealthResponse{
				Enabled: true, HasLeader: true, Healthy: false,
				HealthyVoters: 1, TotalVoters: 2, FailureTolerance: 0,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeClusterHealth(tc.state)
			if got != tc.want {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
