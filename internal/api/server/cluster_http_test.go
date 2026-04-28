// Copyright 2026 Ella Networks

package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func clusterFreePort(t *testing.T) int {
	t.Helper()

	lc := net.ListenConfig{}

	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}

	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	return port
}

func TestClusterHTTP_Status(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	serverPort := clusterFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)

	serverLn := listener.New(listener.Config{
		BindAddress:      serverAddr,
		AdvertiseAddress: serverAddr,
		NodeID:           1,
		TrustBundle:      pki.BundleFunc(),

		Leaf: pki.LeafFunc(1),

		Revoked: func(*big.Int) bool { return false },
	})

	dbPath := filepath.Join(t.TempDir(), "test.db")

	testDB, err := db.NewDatabase(context.Background(), dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	stopCluster := server.StartClusterHTTP(testDB, serverLn)
	defer stopCluster()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := serverLn.Start(ctx); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	// Node 2 dials the cluster port as a peer.
	clientLn := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           2,
		TrustBundle:      pki.BundleFunc(),

		Leaf: pki.LeafFunc(2),

		Revoked: func(*big.Int) bool { return false },
	})

	client := &http.Client{
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return clientLn.Dial(ctx, addr, 1, listener.ALPNHTTP, 5*time.Second)
			},
		},
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet,
		fmt.Sprintf("https://%s/cluster/status", serverAddr), nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /cluster/status: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Result struct {
			Cluster struct {
				Role          string `json:"role"`
				NodeID        int    `json:"nodeId"`
				SchemaVersion int    `json:"schemaVersion"`
			} `json:"cluster"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Standalone DB (no raft manager) returns "Leader" — see db.Database.RaftState().
	if body.Result.Cluster.Role != "Leader" {
		t.Fatalf("expected role %q, got %q", "Leader", body.Result.Cluster.Role)
	}

	if body.Result.Cluster.SchemaVersion == 0 {
		t.Fatal("expected non-zero schema version")
	}

	serverLn.Stop()
}

// clusterTestServer spins up a cluster HTTP server under a listener and
// returns the server's advertise address and a per-peer-node-id client
// factory. Callers close the returned cleanup func.
// clusterTestServerNodeID is the node-id of the server-side listener
// in clusterTestServer. Hardcoded because every test wires the peer
// trust the same way; if a test ever needs a different server node-id,
// this becomes a parameter again.
const clusterTestServerNodeID = 1

func clusterTestServer(t *testing.T, pki *testutil.PKI, peerNodeIDs []int) (serverAddr string, clients map[int]*http.Client, cleanup func()) {
	t.Helper()

	port := clusterFreePort(t)
	serverAddr = fmt.Sprintf("127.0.0.1:%d", port)

	serverLn := listener.New(listener.Config{
		BindAddress:      serverAddr,
		AdvertiseAddress: serverAddr,
		NodeID:           clusterTestServerNodeID,
		TrustBundle:      pki.BundleFunc(),

		Leaf: pki.LeafFunc(clusterTestServerNodeID),

		Revoked: func(*big.Int) bool { return false },
	})

	dbPath := filepath.Join(t.TempDir(), "test.db")

	testDB, err := db.NewDatabase(context.Background(), dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	stopCluster := server.StartClusterHTTP(testDB, serverLn)

	ctx, cancel := context.WithCancel(context.Background())

	if err := serverLn.Start(ctx); err != nil {
		cancel()
		stopCluster()
		t.Fatalf("start listener: %v", err)
	}

	clients = make(map[int]*http.Client, len(peerNodeIDs))

	for _, id := range peerNodeIDs {
		clientLn := listener.New(listener.Config{
			BindAddress:      "127.0.0.1:0",
			AdvertiseAddress: "127.0.0.1:0",
			NodeID:           id,
			TrustBundle:      pki.BundleFunc(),

			Leaf: pki.LeafFunc(id),

			Revoked: func(*big.Int) bool { return false },
		})

		clients[id] = &http.Client{
			Transport: &http.Transport{
				DialTLSContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
					return clientLn.Dial(ctx, addr, clusterTestServerNodeID, listener.ALPNHTTP, 5*time.Second)
				},
			},
			Timeout: 5 * time.Second,
		}
	}

	cleanup = func() {
		cancel()
		stopCluster()
		serverLn.Stop()
	}

	return serverAddr, clients, cleanup
}

// TestClusterHTTP_SelfRegistrationMismatch verifies that a peer whose
// cert CN encodes node-id 5 cannot register as node-id 3 via
// POST /cluster/members on the cluster port.
func TestClusterHTTP_SelfRegistrationMismatch(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 5})

	serverAddr, clients, cleanup := clusterTestServer(t, pki, []int{5})
	defer cleanup()

	body := `{"nodeId":3,"raftAddress":"127.0.0.1:9000","apiAddress":"127.0.0.1:9001"}`

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		fmt.Sprintf("https://%s/cluster/members", serverAddr), strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := clients[5].Do(req)
	if err != nil {
		t.Fatalf("POST /cluster/members: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for nodeId mismatch, got %d", resp.StatusCode)
	}
}

// TestClusterHTTP_SelfAnnounceAccepted verifies the happy path: a peer with
// matching CN successfully refreshes its row via POST /cluster/members/self
// against a standalone DB (db.IsLeader returns true when no raft manager is
// attached).
func TestClusterHTTP_SelfAnnounceAccepted(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 5})

	serverAddr, clients, cleanup := clusterTestServer(t, pki, []int{5})
	defer cleanup()

	body := `{"nodeId":5,"raftAddress":"127.0.0.1:9000","apiAddress":"127.0.0.1:9001","binaryVersion":"abc","maxSchemaVersion":9}`

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		fmt.Sprintf("https://%s/cluster/members/self", serverAddr), strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := clients[5].Do(req)
	if err != nil {
		t.Fatalf("POST /cluster/members/self: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for valid self-announce, got %d", resp.StatusCode)
	}
}

// TestClusterHTTP_SelfAnnounceCNMismatch verifies that the selfRegistrationGuard
// wrapping POST /cluster/members/self rejects a peer announcing a nodeId that
// doesn't match its cert CN, even though the underlying handler (which would
// otherwise upsert on the leader) never runs.
func TestClusterHTTP_SelfAnnounceCNMismatch(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 5})

	serverAddr, clients, cleanup := clusterTestServer(t, pki, []int{5})
	defer cleanup()

	body := `{"nodeId":3,"raftAddress":"127.0.0.1:9000","apiAddress":"127.0.0.1:9001","binaryVersion":"abc","maxSchemaVersion":10}`

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		fmt.Sprintf("https://%s/cluster/members/self", serverAddr), strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := clients[5].Do(req)
	if err != nil {
		t.Fatalf("POST /cluster/members/self: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for CN mismatch, got %d", resp.StatusCode)
	}
}

// TestClusterHTTP_AddMemberRejectsStaleSchema verifies the leader-side
// half of the schema handshake at api_cluster.go: a joiner POSTing to
// /cluster/members with a schemaVersion lower than the leader's binary
// must be refused with 409 Conflict. Without this guard, an old-binary
// node could join a cluster whose applied schema is newer than what it
// supports, and immediately miss migrations the rest of the cluster has
// already applied.
//
// The follower-side counterpart (discovery skipping a peer whose schema
// is higher than the joiner's own) lives in discovery.go and is exercised
// implicitly by the existing TestProbePeer_* tests reading the schema
// field; the integration suite catches end-to-end mismatches.
func TestClusterHTTP_AddMemberRejectsStaleSchema(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 5})

	serverAddr, clients, cleanup := clusterTestServer(t, pki, []int{5})
	defer cleanup()

	staleSchema := db.SchemaVersion() - 1
	if staleSchema < 1 {
		t.Skipf("db.SchemaVersion()=%d too low to construct a stale request", db.SchemaVersion())
	}

	body := fmt.Sprintf(
		`{"nodeId":5,"raftAddress":"127.0.0.1:9000","apiAddress":"127.0.0.1:9001","schemaVersion":%d}`,
		staleSchema,
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		fmt.Sprintf("https://%s/cluster/members", serverAddr), strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := clients[5].Do(req)
	if err != nil {
		t.Fatalf("POST /cluster/members: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for stale schemaVersion, got %d", resp.StatusCode)
	}

	var envelope struct {
		Error string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode error body: %v", err)
	}

	if !strings.Contains(strings.ToLower(envelope.Error), "schema version mismatch") {
		t.Fatalf("expected error to mention schema version mismatch, got %q", envelope.Error)
	}
}
