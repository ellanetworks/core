// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"testing"

	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func voter(id string) ellaraft.Server {
	return ellaraft.Server{NodeID: id, Address: id + ":7000", Suffrage: "voter"}
}

func nonvoter(id string) ellaraft.Server {
	return ellaraft.Server{NodeID: id, Address: id + ":7000", Suffrage: "nonvoter"}
}

func ids(servers []ellaraft.Server) []string {
	out := make([]string, 0, len(servers))
	for _, s := range servers {
		out = append(out, s.NodeID)
	}

	return out
}

func equal(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

func TestRankTransferCandidates(t *testing.T) {
	for _, tc := range []struct {
		name    string
		self    string
		servers []ellaraft.Server
		members []ClusterMember
		health  map[string]bool
		want    []string
	}{
		{
			name:    "sole voter has nobody to hand off to",
			self:    "a",
			servers: []ellaraft.Server{voter("a")},
			want:    nil,
		},
		{
			name:    "nonvoters are never candidates",
			self:    "a",
			servers: []ellaraft.Server{voter("a"), nonvoter("b")},
			want:    nil,
		},
		{
			name:    "a draining peer is never chosen",
			self:    "a",
			servers: []ellaraft.Server{voter("a"), voter("b"), voter("c")},
			members: []ClusterMember{
				{NodeID: "b", DrainState: DrainStateDraining},
				{NodeID: "c", DrainState: DrainStateActive},
			},
			want: []string{"c"},
		},
		{
			name:    "a drained peer is never chosen",
			self:    "a",
			servers: []ellaraft.Server{voter("a"), voter("b"), voter("c")},
			members: []ClusterMember{
				{NodeID: "b", DrainState: DrainStateDrained},
				{NodeID: "c", DrainState: DrainStateDrained},
			},
			want: nil,
		},
		{
			name:    "an empty drain state means active",
			self:    "a",
			servers: []ellaraft.Server{voter("a"), voter("b")},
			members: []ClusterMember{{NodeID: "b"}},
			want:    []string{"b"},
		},
		{
			name:    "a member row missing entirely does not disqualify",
			self:    "a",
			servers: []ellaraft.Server{voter("a"), voter("b")},
			members: nil,
			want:    []string{"b"},
		},
		{
			name:    "healthy voters come before unjudged and unhealthy ones",
			self:    "a",
			servers: []ellaraft.Server{voter("a"), voter("b"), voter("c"), voter("d")},
			health:  map[string]bool{"b": false, "d": true},
			want:    []string{"d", "c", "b"},
		},
		{
			name:    "drain beats health: an unhealthy active peer outranks a healthy draining one",
			self:    "a",
			servers: []ellaraft.Server{voter("a"), voter("b"), voter("c")},
			members: []ClusterMember{
				{NodeID: "b", DrainState: DrainStateDraining},
				{NodeID: "c", DrainState: DrainStateActive},
			},
			health: map[string]bool{"b": true, "c": false},
			want:   []string{"c"},
		},
		{
			name:    "ordering is stable by node id within a tier",
			self:    "a",
			servers: []ellaraft.Server{voter("a"), voter("d"), voter("b"), voter("c")},
			health:  map[string]bool{"d": true, "b": true, "c": true},
			want:    []string{"b", "c", "d"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(rankTransferCandidates(tc.self, tc.servers, tc.members, tc.health))
			if !equal(got, tc.want) {
				t.Fatalf("candidates = %v, want %v", got, tc.want)
			}
		})
	}
}
