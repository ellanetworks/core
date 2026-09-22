// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"testing"
)

func TestDrainingVotersAreNeverTransferTargets(t *testing.T) {
	for _, tc := range []struct {
		name         string
		members      []ClusterMember
		wantEligible []string
		wantExcluded bool
	}{
		{
			name:         "no drain state means every peer is eligible",
			members:      nil,
			wantEligible: []string{"b", "c"},
		},
		{
			name:         "an empty drain state means active",
			members:      []ClusterMember{{NodeID: "b"}, {NodeID: "c"}},
			wantEligible: []string{"b", "c"},
		},
		{
			name: "a draining peer is excluded",
			members: []ClusterMember{
				{NodeID: "b", DrainState: DrainStateDraining},
				{NodeID: "c", DrainState: DrainStateActive},
			},
			wantEligible: []string{"c"},
			wantExcluded: true,
		},
		{
			name: "a drained peer is excluded",
			members: []ClusterMember{
				{NodeID: "b", DrainState: DrainStateDrained},
				{NodeID: "c", DrainState: DrainStateDrained},
			},
			wantEligible: nil,
			wantExcluded: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			draining := make(map[string]bool, len(tc.members))
			for _, m := range tc.members {
				draining[m.NodeID] = normalizeDrainState(m.DrainState) != DrainStateActive
			}

			var eligible []string

			excluded := false

			for _, id := range []string{"a", "b", "c"} {
				if id == "a" {
					continue
				}

				if draining[id] {
					excluded = true
					continue
				}

				eligible = append(eligible, id)
			}

			if excluded != tc.wantExcluded {
				t.Fatalf("excluded = %v, want %v", excluded, tc.wantExcluded)
			}

			if len(eligible) != len(tc.wantEligible) {
				t.Fatalf("eligible = %v, want %v", eligible, tc.wantEligible)
			}

			for i := range eligible {
				if eligible[i] != tc.wantEligible[i] {
					t.Fatalf("eligible = %v, want %v", eligible, tc.wantEligible)
				}
			}
		})
	}
}
