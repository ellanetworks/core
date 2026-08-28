// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"strconv"
	"testing"

	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
)

type quorumTestServer struct {
	id      int
	state   autopilot.RaftState
	healthy bool
}

func quorumTestState(servers ...quorumTestServer) *autopilot.State {
	state := &autopilot.State{
		Servers: make(map[raft.ServerID]*autopilot.ServerState, len(servers)),
	}

	for _, s := range servers {
		id := raft.ServerID(strconv.Itoa(s.id))
		state.Servers[id] = &autopilot.ServerState{
			Server: autopilot.Server{ID: id},
			State:  s.state,
			Health: autopilot.ServerHealth{Healthy: s.healthy},
		}
	}

	return state
}

func TestQuorumSafeToRemove(t *testing.T) {
	tests := []struct {
		name    string
		servers []quorumTestServer
		target  int
		want    bool
	}{
		{
			name: "three healthy voters, remove one",
			servers: []quorumTestServer{
				{1, autopilot.RaftLeader, true},
				{2, autopilot.RaftVoter, true},
				{3, autopilot.RaftVoter, true},
			},
			target: 3,
			want:   true,
		},
		{
			name: "three voters one down, remove a healthy one",
			servers: []quorumTestServer{
				{1, autopilot.RaftLeader, true},
				{2, autopilot.RaftVoter, true},
				{3, autopilot.RaftVoter, false},
			},
			target: 2,
			want:   false,
		},
		{
			name: "three voters one down, remove the down one",
			servers: []quorumTestServer{
				{1, autopilot.RaftLeader, true},
				{2, autopilot.RaftVoter, true},
				{3, autopilot.RaftVoter, false},
			},
			target: 3,
			want:   true,
		},
		{
			name: "five voters two down, remove a healthy one",
			servers: []quorumTestServer{
				{1, autopilot.RaftLeader, true},
				{2, autopilot.RaftVoter, true},
				{3, autopilot.RaftVoter, true},
				{4, autopilot.RaftVoter, false},
				{5, autopilot.RaftVoter, false},
			},
			target: 2,
			want:   false,
		},
		{
			name: "five healthy voters, remove one",
			servers: []quorumTestServer{
				{1, autopilot.RaftLeader, true},
				{2, autopilot.RaftVoter, true},
				{3, autopilot.RaftVoter, true},
				{4, autopilot.RaftVoter, true},
				{5, autopilot.RaftVoter, true},
			},
			target: 5,
			want:   true,
		},
		{
			name: "nonvoter target does not shrink the voter set",
			servers: []quorumTestServer{
				{1, autopilot.RaftLeader, true},
				{2, autopilot.RaftVoter, true},
				{3, autopilot.RaftVoter, false},
				{4, autopilot.RaftNonVoter, true},
			},
			target: 4,
			want:   true,
		},
		{
			name: "two healthy voters, remove one",
			servers: []quorumTestServer{
				{1, autopilot.RaftLeader, true},
				{2, autopilot.RaftVoter, true},
			},
			target: 2,
			want:   true,
		},
		{
			name: "target absent from the raft configuration is a no-op removal",
			servers: []quorumTestServer{
				{1, autopilot.RaftLeader, true},
				{2, autopilot.RaftVoter, true},
				{3, autopilot.RaftVoter, false},
			},
			target: 99,
			want:   true,
		},
		{
			name: "four healthy voters, remove one",
			servers: []quorumTestServer{
				{1, autopilot.RaftLeader, true},
				{2, autopilot.RaftVoter, true},
				{3, autopilot.RaftVoter, true},
				{4, autopilot.RaftVoter, true},
			},
			target: 4,
			want:   true,
		},
		{
			name: "four voters one down, remove a healthy one",
			servers: []quorumTestServer{
				{1, autopilot.RaftLeader, true},
				{2, autopilot.RaftVoter, true},
				{3, autopilot.RaftVoter, true},
				{4, autopilot.RaftVoter, false},
			},
			target: 3,
			want:   true,
		},
		{
			name: "four voters two down, remove a healthy one",
			servers: []quorumTestServer{
				{1, autopilot.RaftLeader, true},
				{2, autopilot.RaftVoter, true},
				{3, autopilot.RaftVoter, false},
				{4, autopilot.RaftVoter, false},
			},
			target: 2,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quorumSafeToRemove(quorumTestState(tt.servers...), tt.target)
			if got != tt.want {
				t.Errorf("quorumSafeToRemove(target=%d) = %v, want %v", tt.target, got, tt.want)
			}
		})
	}
}
