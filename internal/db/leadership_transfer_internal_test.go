// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"slices"
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func TestDrainingVotersAreNeverTransferTargets(t *testing.T) {
	voters := []ellaraft.Server{
		{NodeID: "a", Suffrage: "voter"},
		{NodeID: "b", Suffrage: "voter"},
		{NodeID: "c", Suffrage: "voter"},
	}

	for _, tc := range []struct {
		name         string
		servers      []ellaraft.Server
		members      []ClusterMember
		wantEligible []string
		wantExcluded bool
	}{
		{
			name:         "no drain state means every peer is eligible",
			servers:      voters,
			members:      nil,
			wantEligible: []string{"b", "c"},
		},
		{
			name:         "an empty drain state means active",
			servers:      voters,
			members:      []ClusterMember{{NodeID: "b"}, {NodeID: "c"}},
			wantEligible: []string{"b", "c"},
		},
		{
			name:    "a draining peer is excluded",
			servers: voters,
			members: []ClusterMember{
				{NodeID: "b", DrainState: DrainStateDraining},
				{NodeID: "c", DrainState: DrainStateActive},
			},
			wantEligible: []string{"c"},
			wantExcluded: true,
		},
		{
			name:    "a drained peer is excluded",
			servers: voters,
			members: []ClusterMember{
				{NodeID: "b", DrainState: DrainStateDrained},
				{NodeID: "c", DrainState: DrainStateDrained},
			},
			wantEligible: nil,
			wantExcluded: true,
		},
		{
			name: "a nonvoter is never a target",
			servers: []ellaraft.Server{
				{NodeID: "a", Suffrage: "voter"},
				{NodeID: "b", Suffrage: "nonvoter"},
				{NodeID: "c", Suffrage: "voter"},
			},
			wantEligible: []string{"c"},
		},
		{
			name:         "self is never a target",
			servers:      []ellaraft.Server{{NodeID: "a", Suffrage: "voter"}},
			wantEligible: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, excluded := selectTransferCandidates(tc.servers, "a", tc.members)

			if excluded != tc.wantExcluded {
				t.Fatalf("excluded = %v, want %v", excluded, tc.wantExcluded)
			}

			var eligible []string
			for _, srv := range got {
				eligible = append(eligible, srv.NodeID)
			}

			if !slices.Equal(eligible, tc.wantEligible) {
				t.Fatalf("eligible = %v, want %v", eligible, tc.wantEligible)
			}
		})
	}
}
