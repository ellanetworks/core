package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
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

	t.Run("3. Negative schemaVersion is rejected", func(t *testing.T) {
		body := `{"nodeId": 5, "raftAddress": "10.0.0.5:7000", "apiAddress": "https://10.0.0.5:5000", "schemaVersion": -1}`

		statusCode, err := addClusterMember(env.Server.URL, client, token, body)
		if err != nil {
			t.Fatalf("request failed: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for negative schemaVersion, got %d", statusCode)
		}
	})

	t.Run("4. Negative maxSchemaVersion is rejected", func(t *testing.T) {
		body := `{"nodeId": 5, "raftAddress": "10.0.0.5:7000", "apiAddress": "https://10.0.0.5:5000", "maxSchemaVersion": -1}`

		statusCode, err := addClusterMember(env.Server.URL, client, token, body)
		if err != nil {
			t.Fatalf("request failed: %s", err)
		}

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for negative maxSchemaVersion, got %d", statusCode)
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

func getClusterMember(url string, client *http.Client, token string, nodeID int) (int, *ClusterMemberResponseItem, string, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET",
		fmt.Sprintf("%s/api/v1/cluster/members/%d", url, nodeID), nil)
	if err != nil {
		return 0, nil, "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, "", err
	}

	defer func() { _ = res.Body.Close() }()

	var body struct {
		Error  string                     `json:"error,omitempty"`
		Result *ClusterMemberResponseItem `json:"result,omitempty"`
	}

	_ = json.NewDecoder(res.Body).Decode(&body)

	return res.StatusCode, body.Result, body.Error, nil
}

func TestGetClusterMember(t *testing.T) {
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

	t.Run("not found", func(t *testing.T) {
		status, _, msg, err := getClusterMember(env.Server.URL, client, token, 99)
		if err != nil {
			t.Fatalf("request failed: %s", err)
		}

		if status != http.StatusNotFound {
			t.Fatalf("expected 404, got %d (body: %s)", status, msg)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(context.Background(), "GET",
			env.Server.URL+"/api/v1/cluster/members/not-a-number", nil)

		req.Header.Set("Authorization", "Bearer "+token)

		res, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %s", err)
		}

		defer func() { _ = res.Body.Close() }()

		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", res.StatusCode)
		}
	})

	t.Run("returns member", func(t *testing.T) {
		member := &db.ClusterMember{
			NodeID:           7,
			RaftAddress:      "10.0.0.7:7000",
			APIAddress:       "https://10.0.0.7:5000",
			BinaryVersion:    "v1.2.3",
			Suffrage:         "nonvoter",
			MaxSchemaVersion: 9,
		}

		if err := env.DB.UpsertClusterMember(context.Background(), member); err != nil {
			t.Fatalf("upsert: %s", err)
		}

		status, result, msg, err := getClusterMember(env.Server.URL, client, token, 7)
		if err != nil {
			t.Fatalf("request failed: %s", err)
		}

		if status != http.StatusOK {
			t.Fatalf("expected 200, got %d (body: %s)", status, msg)
		}

		if result == nil || result.NodeID != 7 || result.RaftAddress != "10.0.0.7:7000" {
			t.Fatalf("unexpected result: %+v", result)
		}

		if result.IsLeader {
			t.Errorf("expected isLeader=false for member 7 (leader is node 1)")
		}
	})
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

// TestRemoveClusterMember_PurgesDynamicLeases verifies that removing a
// cluster member also purges the dynamic IP leases owned by that node.
// Without this, the removed node's addresses would remain marked "in
// use" in the replicated lease table forever, slowly draining the pool.
// Static leases (admin-pinned) must be preserved.
func TestRemoveClusterMember_PurgesDynamicLeases(t *testing.T) {
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

	ctx := context.Background()

	// Look up the default data network's pool + default profile so we
	// have valid foreign-key targets for the seeded subscribers and
	// leases.
	dn, err := env.DB.GetDataNetwork(ctx, db.InitialDataNetworkName)
	if err != nil {
		t.Fatalf("get default data network: %s", err)
	}

	profile, err := env.DB.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatalf("get default profile: %s", err)
	}

	const removedNodeID = 42

	// Seed a cluster_members row for the node we're about to remove.
	// Using a distinct IP so the leader-removal guard doesn't fire.
	if err := env.DB.UpsertClusterMember(ctx, &db.ClusterMember{
		NodeID:        removedNodeID,
		RaftAddress:   "10.0.0.42:7000",
		APIAddress:    "http://10.0.0.42:5000",
		BinaryVersion: "test",
		Suffrage:      "voter",
	}); err != nil {
		t.Fatalf("upsert cluster member: %s", err)
	}

	// Seed two dynamic leases owned by the node being removed (these
	// must be purged) and one static lease (which must survive).
	// Leases reference subscribers via a FK on IMSI, so the subscriber
	// rows are created first.
	seedLease := func(addr string, imsi string, leaseType string) {
		t.Helper()

		if err := env.DB.CreateSubscriber(ctx, &db.Subscriber{
			Imsi:           imsi,
			SequenceNumber: "000000000001",
			PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
			Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
			ProfileID:      profile.ID,
		}); err != nil {
			t.Fatalf("create subscriber %s: %s", imsi, err)
		}

		parsed, parseErr := netip.ParseAddr(addr)
		if parseErr != nil {
			t.Fatalf("parse %s: %s", addr, parseErr)
		}

		sessionID := 1

		if err := env.DB.CreateLease(ctx, &db.IPLease{
			PoolID:    dn.ID,
			IMSI:      imsi,
			SessionID: &sessionID,
			Type:      leaseType,
			CreatedAt: 1,
			NodeID:    removedNodeID,
		}, parsed); err != nil {
			t.Fatalf("create lease %s: %s", addr, err)
		}
	}

	seedLease("10.45.0.10", "001010000000001", "dynamic")
	seedLease("10.45.0.11", "001010000000002", "dynamic")
	seedLease("10.45.0.12", "001010000000003", "static")

	// Call DELETE /api/v1/cluster/members/42.
	status, msg, err := removeClusterMember(env.Server.URL, client, token, removedNodeID)
	if err != nil {
		t.Fatalf("request failed: %s", err)
	}

	if status != http.StatusOK {
		t.Fatalf("expected 200 on remove, got %d (body: %s)", status, msg)
	}

	// Dynamic leases for node 42 must be gone; the static lease must
	// still be present.
	leases, err := env.DB.ListActiveLeasesByNode(ctx, removedNodeID)
	if err != nil {
		t.Fatalf("list leases: %s", err)
	}

	var dyn, stat int

	for _, l := range leases {
		switch l.Type {
		case "dynamic":
			dyn++
		case "static":
			stat++
		}
	}

	if dyn != 0 {
		t.Errorf("expected 0 dynamic leases after remove, got %d", dyn)
	}

	if stat != 1 {
		t.Errorf("expected 1 surviving static lease, got %d", stat)
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
