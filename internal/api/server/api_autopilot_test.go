package server

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
)

// NOTE: End-to-end HTTP tests of the /api/v1/cluster/autopilot route
// live in api_autopilot_http_test.go (black-box server_test package)
// where the test harness is available.

func TestMapAutopilotState_Nil(t *testing.T) {
	resp := mapAutopilotState(nil)

	if resp.Healthy {
		t.Errorf("expected Healthy=false for nil state, got true")
	}

	if resp.FailureTolerance != 0 {
		t.Errorf("expected FailureTolerance=0, got %d", resp.FailureTolerance)
	}

	if resp.LeaderNodeID != 0 {
		t.Errorf("expected LeaderNodeID=0, got %d", resp.LeaderNodeID)
	}

	if resp.Voters == nil {
		t.Errorf("expected non-nil empty Voters slice for stable JSON marshaling")
	}

	if resp.Servers == nil {
		t.Errorf("expected non-nil empty Servers slice for stable JSON marshaling")
	}

	// JSON must encode empty slices as [], not null — the UI relies on this.
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got := string(b)

	for _, substr := range []string{`"voters":[]`, `"servers":[]`} {
		if !contains(got, substr) {
			t.Errorf("expected JSON to contain %s, got %s", substr, got)
		}
	}
}

func TestMapAutopilotState_Populated(t *testing.T) {
	stable := time.Date(2026, 4, 20, 8, 15, 2, 0, time.UTC)

	state := &autopilot.State{
		Healthy:          true,
		FailureTolerance: 1,
		Leader:           raft.ServerID("1"),
		Voters:           []raft.ServerID{"3", "1", "2"},
		Servers: map[raft.ServerID]*autopilot.ServerState{
			"2": {
				Server: autopilot.Server{
					ID:         raft.ServerID("2"),
					Address:    raft.ServerAddress("10.0.0.2:7000"),
					NodeStatus: autopilot.NodeAlive,
					IsLeader:   false,
				},
				State:  autopilot.RaftVoter,
				Stats:  autopilot.ServerStats{LastContact: 12 * time.Millisecond, LastTerm: 7, LastIndex: 18233},
				Health: autopilot.ServerHealth{Healthy: true, StableSince: stable},
			},
			"1": {
				Server: autopilot.Server{
					ID:         raft.ServerID("1"),
					Address:    raft.ServerAddress("10.0.0.1:7000"),
					NodeStatus: autopilot.NodeAlive,
					IsLeader:   true,
				},
				State:  autopilot.RaftLeader,
				Stats:  autopilot.ServerStats{LastContact: 0, LastTerm: 7, LastIndex: 18234},
				Health: autopilot.ServerHealth{Healthy: true, StableSince: stable},
			},
			"3": {
				Server: autopilot.Server{
					ID:         raft.ServerID("3"),
					Address:    raft.ServerAddress("10.0.0.3:7000"),
					NodeStatus: autopilot.NodeLeft,
					IsLeader:   false,
				},
				State:  autopilot.RaftVoter,
				Stats:  autopilot.ServerStats{LastContact: 30 * time.Second, LastTerm: 7, LastIndex: 17500},
				Health: autopilot.ServerHealth{Healthy: false, StableSince: stable},
			},
		},
	}

	resp := mapAutopilotState(state)

	if !resp.Healthy {
		t.Errorf("expected Healthy=true")
	}

	if resp.FailureTolerance != 1 {
		t.Errorf("expected FailureTolerance=1, got %d", resp.FailureTolerance)
	}

	if resp.LeaderNodeID != 1 {
		t.Errorf("expected LeaderNodeID=1, got %d", resp.LeaderNodeID)
	}

	// Voters must be sorted and integer-decoded.
	if got := resp.Voters; !intSliceEqual(got, []int{1, 2, 3}) {
		t.Errorf("expected Voters=[1 2 3], got %v", got)
	}

	// Servers must be sorted by nodeId.
	if len(resp.Servers) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(resp.Servers))
	}

	for i, want := range []int{1, 2, 3} {
		if resp.Servers[i].NodeID != want {
			t.Errorf("server[%d]: expected NodeID=%d, got %d", i, want, resp.Servers[i].NodeID)
		}
	}

	leader := resp.Servers[0]
	if !leader.IsLeader || !leader.Healthy || !leader.HasVotingRights {
		t.Errorf("leader row should be leader+healthy+voting, got %+v", leader)
	}

	departed := resp.Servers[2]
	if departed.Healthy {
		t.Errorf("expected node 3 unhealthy")
	}

	if departed.NodeStatus != string(autopilot.NodeLeft) {
		t.Errorf("expected node 3 nodeStatus=%q, got %q", autopilot.NodeLeft, departed.NodeStatus)
	}

	if leader.StableSince != "2026-04-20T08:15:02Z" {
		t.Errorf("expected stableSince RFC3339 UTC, got %q", leader.StableSince)
	}
}

func TestMapAutopilotState_SkipsMalformedIDs(t *testing.T) {
	state := &autopilot.State{
		Leader: raft.ServerID("not-a-number"),
		Voters: []raft.ServerID{"1", "not-a-number", "2"},
		Servers: map[raft.ServerID]*autopilot.ServerState{
			"not-a-number": {
				Server: autopilot.Server{ID: raft.ServerID("not-a-number")},
			},
			"1": {
				Server: autopilot.Server{ID: raft.ServerID("1"), IsLeader: true},
			},
		},
	}

	resp := mapAutopilotState(state)

	if resp.LeaderNodeID != 0 {
		t.Errorf("expected LeaderNodeID=0 for malformed id, got %d", resp.LeaderNodeID)
	}

	if !intSliceEqual(resp.Voters, []int{1, 2}) {
		t.Errorf("expected malformed voter dropped, got %v", resp.Voters)
	}

	if len(resp.Servers) != 1 || resp.Servers[0].NodeID != 1 {
		t.Errorf("expected single server row for node 1, got %+v", resp.Servers)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

func intSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
