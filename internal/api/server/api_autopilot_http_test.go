package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
)

type autopilotServerItem struct {
	NodeID          int    `json:"nodeId"`
	RaftAddress     string `json:"raftAddress"`
	NodeStatus      string `json:"nodeStatus"`
	Healthy         bool   `json:"healthy"`
	IsLeader        bool   `json:"isLeader"`
	HasVotingRights bool   `json:"hasVotingRights"`
	StableSince     string `json:"stableSince,omitempty"`
}

type autopilotStateResult struct {
	Healthy          bool                  `json:"healthy"`
	FailureTolerance int                   `json:"failureTolerance"`
	LeaderNodeID     int                   `json:"leaderNodeId"`
	Voters           []int                 `json:"voters"`
	Servers          []autopilotServerItem `json:"servers"`
}

type autopilotStateResponse struct {
	Error  string               `json:"error,omitempty"`
	Result autopilotStateResult `json:"result"`
}

func getAutopilotState(url string, client *http.Client, token string) (int, *autopilotStateResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/cluster/autopilot", nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() { _ = res.Body.Close() }()

	var body autopilotStateResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &body, nil
}

// TestGetAutopilotState_NoCluster verifies that on a node where HA is not
// enabled (the test harness default), the endpoint still responds 200 with
// an empty envelope. Autopilot only runs on the leader of a real cluster;
// returning an empty state here keeps the UI contract consistent.
func TestGetAutopilotState_NoCluster(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	status, body, err := getAutopilotState(env.Server.URL, client, token)
	if err != nil {
		t.Fatalf("couldn't get autopilot state: %s", err)
	}

	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %+v", status, body)
	}

	if body.Result.Healthy {
		t.Errorf("expected healthy=false without a running cluster")
	}

	if body.Result.FailureTolerance != 0 {
		t.Errorf("expected failureTolerance=0, got %d", body.Result.FailureTolerance)
	}

	if body.Result.LeaderNodeID != 0 {
		t.Errorf("expected leaderNodeId=0, got %d", body.Result.LeaderNodeID)
	}

	// Slices must be present (not nil) so the UI can iterate without guards.
	if body.Result.Voters == nil {
		t.Errorf("expected non-nil voters array")
	}

	if body.Result.Servers == nil {
		t.Errorf("expected non-nil servers array")
	}
}

func TestGetAutopilotState_Unauthorized(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	if _, err := initializeAndRefresh(env.Server.URL, client); err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	status, _, err := getAutopilotState(env.Server.URL, client, "")
	if err != nil {
		t.Fatalf("request failed: %s", err)
	}

	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", status)
	}
}
