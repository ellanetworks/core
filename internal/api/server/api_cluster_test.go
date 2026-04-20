package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func removeClusterMember(url string, client *http.Client, token string, nodeID int) (int, string, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE",
		fmt.Sprintf("%s/api/v1/cluster/members/%d", url, nodeID), nil)
	if err != nil {
		return 0, "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}

	defer func() { _ = res.Body.Close() }()

	var body struct {
		Error string `json:"error,omitempty"`
	}

	_ = json.NewDecoder(res.Body).Decode(&body)

	return res.StatusCode, body.Error, nil
}

type ClusterMemberResponseItem struct {
	NodeID      int    `json:"nodeId"`
	RaftAddress string `json:"raftAddress"`
	APIAddress  string `json:"apiAddress"`
	IsLeader    bool   `json:"isLeader"`
}

type ListClusterMembersResponse struct {
	Error  string                      `json:"error,omitempty"`
	Result []ClusterMemberResponseItem `json:"result"`
}

func listClusterMembers(url string, client *http.Client, token string) (int, *ListClusterMembersResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/cluster/members", nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() { _ = res.Body.Close() }()

	var response ListClusterMembersResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func addClusterMember(url string, client *http.Client, token string, body string) (int, error) {
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/cluster/members", strings.NewReader(body))
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	defer func() { _ = res.Body.Close() }()

	return res.StatusCode, nil
}

func TestClusterMembersEndToEnd(t *testing.T) {
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

	t.Run("1. List cluster members (empty)", func(t *testing.T) {
		statusCode, response, err := listClusterMembers(env.Server.URL, client, token)
		if err != nil {
			t.Fatalf("couldn't list cluster members: %s", err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if len(response.Result) != 0 {
			t.Fatalf("expected 0 members, got %d", len(response.Result))
		}
	})

	t.Run("2. Add cluster member with invalid body", func(t *testing.T) {
		statusCode, err := addClusterMember(env.Server.URL, client, token, `{"nodeId": 0}`)
		if err != nil {
			t.Fatalf("couldn't add cluster member: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}
	})
}

func TestListClusterMembers_IncludesHAFields(t *testing.T) {
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

	member := &db.ClusterMember{
		NodeID:           7,
		RaftAddress:      "10.0.0.7:7000",
		APIAddress:       "10.0.0.7:8443",
		BinaryVersion:    "v1.2.3",
		Suffrage:         "voter",
		MaxSchemaVersion: 1,
	}

	if err := env.DB.UpsertClusterMember(context.Background(), member); err != nil {
		t.Fatalf("couldn't upsert cluster member: %s", err)
	}

	statusCode, response, err := listClusterMembers(env.Server.URL, client, token)
	if err != nil {
		t.Fatalf("couldn't list cluster members: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if len(response.Result) != 1 {
		t.Fatalf("expected 1 member, got %d", len(response.Result))
	}

	m := response.Result[0]
	if m.NodeID != 7 {
		t.Fatalf("expected nodeId 7, got %d", m.NodeID)
	}

	// Test server runs without a Raft cluster, so no leader is known.
	if m.IsLeader {
		t.Fatalf("expected isLeader=false with no cluster, got true")
	}
}

func TestRemoveClusterMember_NotFound(t *testing.T) {
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

	status, msg, err := removeClusterMember(env.Server.URL, client, token, 99)
	if err != nil {
		t.Fatalf("request failed: %s", err)
	}

	if status != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown nodeId, got %d (body: %s)", status, msg)
	}

	if !strings.Contains(strings.ToLower(msg), "not found") {
		t.Errorf("expected body to mention not found, got %q", msg)
	}
}

// TestRemoveClusterMember_RefusesLeader verifies the server rejects a
// remove call targeting the current leader with 409. In standalone mode
// (the test harness default) the single node is itself the leader, so
// this double-checks the guard against the most common footgun: an
// operator calling remove against node 1 before draining.
func TestRemoveClusterMember_RefusesLeader(t *testing.T) {
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

	leaderAddr := env.DB.LeaderAddress()
	if leaderAddr == "" {
		t.Fatal("test harness did not elect a leader")
	}

	member := &db.ClusterMember{
		NodeID:        env.DB.NodeID(),
		RaftAddress:   leaderAddr,
		APIAddress:    "http://127.0.0.1:0",
		BinaryVersion: "test",
		Suffrage:      "voter",
	}

	if err := env.DB.UpsertClusterMember(context.Background(), member); err != nil {
		t.Fatalf("upsert cluster member: %s", err)
	}

	status, msg, err := removeClusterMember(env.Server.URL, client, token, member.NodeID)
	if err != nil {
		t.Fatalf("request failed: %s", err)
	}

	if status != http.StatusConflict {
		t.Fatalf("expected 409 when removing leader, got %d (body: %s)", status, msg)
	}

	if !strings.Contains(strings.ToLower(msg), "leader") {
		t.Errorf("expected body to mention leader, got %q", msg)
	}
}

func TestClusterStatusIncluded(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	statusCode, response, err := getStatus(env.Server.URL, client)
	if err != nil {
		t.Fatalf("couldn't get status: %s", err)
	}

	if statusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if response.Result.Version == "" {
		t.Fatalf("expected version to be non-empty")
	}
}
