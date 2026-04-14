package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

type ClusterMemberResponseItem struct {
	NodeID      int    `json:"nodeId"`
	RaftAddress string `json:"raftAddress"`
	APIAddress  string `json:"apiAddress"`
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
