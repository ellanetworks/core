// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"testing"

	"github.com/ellanetworks/core/internal/pki"
)

func TestSummarizeClusterHealth(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		state                AutopilotStateResponse
		configuredVoters     int
		wantState            string
		wantTotalVoters      int
		wantHealthyVoters    int
		wantFailureTolerance int
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
			configuredVoters:     3,
			wantState:            ClusterHealthHealthy,
			wantTotalVoters:      3,
			wantHealthyVoters:    3,
			wantFailureTolerance: 1,
		},
		{
			name: "one voter down",
			state: AutopilotStateResponse{
				Healthy:      false,
				LeaderNodeID: "a",
				Voters:       []pki.NodeID{"a", "b", "c"},
				Servers: []AutopilotServerResponse{
					{NodeID: "a", Healthy: true, HasVotingRights: true},
					{NodeID: "b", Healthy: true, HasVotingRights: true},
					{NodeID: "c", Healthy: false, HasVotingRights: true},
				},
			},
			configuredVoters:  3,
			wantState:         ClusterHealthDegraded,
			wantTotalVoters:   3,
			wantHealthyVoters: 2,
		},
		{
			name: "an unhealthy nonvoter degrades without moving the ratio",
			state: AutopilotStateResponse{
				Healthy:          false,
				FailureTolerance: 1,
				LeaderNodeID:     "a",
				Voters:           []pki.NodeID{"a", "b", "c"},
				Servers: []AutopilotServerResponse{
					{NodeID: "a", Healthy: true, HasVotingRights: true},
					{NodeID: "b", Healthy: true, HasVotingRights: true},
					{NodeID: "c", Healthy: true, HasVotingRights: true},
					{NodeID: "d", Healthy: false, HasVotingRights: false},
				},
			},
			configuredVoters:     3,
			wantState:            ClusterHealthDegraded,
			wantTotalVoters:      3,
			wantHealthyVoters:    3,
			wantFailureTolerance: 1,
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
			configuredVoters:  2,
			wantState:         ClusterHealthDegraded,
			wantTotalVoters:   2,
			wantHealthyVoters: 1,
		},
		{
			name: "an empty voter roster falls back to the local configuration",
			state: AutopilotStateResponse{
				Healthy:      false,
				LeaderNodeID: "a",
				Voters:       []pki.NodeID{},
				Servers:      []AutopilotServerResponse{},
			},
			configuredVoters:  3,
			wantState:         ClusterHealthDegraded,
			wantTotalVoters:   3,
			wantHealthyVoters: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := summarizeClusterHealth(tc.state, tc.configuredVoters)

			if got.State != tc.wantState {
				t.Errorf("State: got %q, want %q", got.State, tc.wantState)
			}

			if got.TotalVoters != tc.wantTotalVoters {
				t.Errorf("TotalVoters: got %d, want %d", got.TotalVoters, tc.wantTotalVoters)
			}

			if got.HealthyVoters == nil || *got.HealthyVoters != tc.wantHealthyVoters {
				t.Errorf("HealthyVoters: got %v, want %d", got.HealthyVoters, tc.wantHealthyVoters)
			}

			if got.FailureTolerance == nil || *got.FailureTolerance != tc.wantFailureTolerance {
				t.Errorf("FailureTolerance: got %v, want %d", got.FailureTolerance, tc.wantFailureTolerance)
			}
		})
	}
}

func TestSingleServerHealth(t *testing.T) {
	for _, tc := range []struct {
		name            string
		voters          int
		wantTotalVoters int
	}{
		{name: "sole voter", voters: 1, wantTotalVoters: 1},
		{name: "unreadable configuration falls back to one", voters: 0, wantTotalVoters: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := singleServerHealth(tc.voters)

			if got.State != ClusterHealthHealthy {
				t.Errorf("State: got %q, want %q", got.State, ClusterHealthHealthy)
			}

			if got.TotalVoters != tc.wantTotalVoters {
				t.Errorf("TotalVoters: got %d, want %d", got.TotalVoters, tc.wantTotalVoters)
			}

			if got.HealthyVoters == nil || *got.HealthyVoters != tc.wantTotalVoters {
				t.Errorf("HealthyVoters: got %v, want %d", got.HealthyVoters, tc.wantTotalVoters)
			}

			if got.FailureTolerance == nil || *got.FailureTolerance != 0 {
				t.Errorf("FailureTolerance: got %v, want 0", got.FailureTolerance)
			}
		})
	}
}
