package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
)

type ClusterStatusBody struct {
	Enabled          bool   `json:"enabled"`
	Role             string `json:"role"`
	NodeID           int    `json:"nodeId"`
	IsLeader         bool   `json:"isLeader"`
	LeaderNodeID     int    `json:"leaderNodeId"`
	LeaderAPIAddress string `json:"leaderAPIAddress,omitempty"`
}

type GetStatusResponseResult struct {
	Version       string             `json:"version"`
	SchemaVersion int                `json:"schemaVersion"`
	Cluster       *ClusterStatusBody `json:"cluster,omitempty"`
}

type GetStatusResponse struct {
	Error  string                  `json:"error,omitempty"`
	Result GetStatusResponseResult `json:"result"`
}

func getStatus(url string, client *http.Client) (int, *GetStatusResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/status", nil)
	if err != nil {
		return 0, nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var radioResponse GetStatusResponse
	if err := json.NewDecoder(res.Body).Decode(&radioResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &radioResponse, nil
}

// This is an end-to-end test for the status handlers.
// The order of the tests is important, as some tests depend on
// the state of the server after previous tests.
func TestStatusEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	env, err := setupServer(dbPath)
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}
	defer env.Server.Close()

	client := newTestClient(env.Server)

	t.Run("1. Get status", func(t *testing.T) {
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

		// schemaVersion is per-binary and must be reported at the top
		// level regardless of whether HA is enabled.
		if response.Result.SchemaVersion <= 0 {
			t.Fatalf("expected positive schemaVersion, got %d", response.Result.SchemaVersion)
		}

		// Test server runs without Raft, so the cluster sub-object is omitted.
		if response.Result.Cluster != nil {
			t.Fatalf("expected cluster to be nil with clustering disabled, got %+v", response.Result.Cluster)
		}
	})
}
