// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	ellapki "github.com/ellanetworks/core/internal/pki"
	ellaraft "github.com/ellanetworks/core/internal/raft"
)

func TestClusterHTTP_Status(t *testing.T) {
	pki := testutil.GenTestPKI(t, []string{"1", "2"})

	serverLn := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           "1",
		Pin:              pki.PinFunc(),

		Leaf: pki.LeafFunc("1"),
	})

	dbPath := filepath.Join(t.TempDir(), "test.db")

	testDB, err := db.NewDatabase(context.Background(), dbPath, ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	if err := testDB.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("database never became ready: %v", err)
	}

	stopCluster := server.StartClusterHTTP(testDB, serverLn, false)
	defer stopCluster()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := serverLn.Start(ctx); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	serverAddr := serverLn.BoundAddress()

	// Node 2 dials the cluster port as a peer.
	clientLn := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           "2",
		Pin:              pki.PinFunc(),

		Leaf: pki.LeafFunc("2"),
	})

	client := &http.Client{
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return clientLn.Dial(ctx, addr, "1", listener.ALPNHTTP, 5*time.Second)
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
				Role          string         `json:"role"`
				NodeID        ellapki.NodeID `json:"nodeId"`
				SchemaVersion int            `json:"schemaVersion"`
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
const clusterTestServerNodeID = "1"

func clusterTestServer(t *testing.T, pki *testutil.PKI, peerNodeIDs []string) (serverAddr string, clients map[string]*http.Client, cleanup func()) {
	t.Helper()

	serverLn := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           clusterTestServerNodeID,
		Pin:              pki.PinFunc(),

		Leaf: pki.LeafFunc(clusterTestServerNodeID),
	})

	dbPath := filepath.Join(t.TempDir(), "test.db")

	testDB, err := db.NewDatabase(context.Background(), dbPath, ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	if err := testDB.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("database never became ready: %v", err)
	}

	stopCluster := server.StartClusterHTTP(testDB, serverLn, false)

	ctx, cancel := context.WithCancel(context.Background())

	if err := serverLn.Start(ctx); err != nil {
		cancel()
		stopCluster()
		t.Fatalf("start listener: %v", err)
	}

	serverAddr = serverLn.BoundAddress()

	clients = make(map[string]*http.Client, len(peerNodeIDs))

	for _, id := range peerNodeIDs {
		clientLn := listener.New(listener.Config{
			BindAddress:      "127.0.0.1:0",
			AdvertiseAddress: "127.0.0.1:0",
			NodeID:           id,
			Pin:              pki.PinFunc(),

			Leaf: pki.LeafFunc(id),
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
	pki := testutil.GenTestPKI(t, []string{"1", "5"})

	serverAddr, clients, cleanup := clusterTestServer(t, pki, []string{"5"})
	defer cleanup()

	body := `{"nodeId":3,"raftAddress":"127.0.0.1:9000","apiAddress":"127.0.0.1:9001"}`

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		fmt.Sprintf("https://%s/cluster/members", serverAddr), strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := clients["5"].Do(req)
	if err != nil {
		t.Fatalf("POST /cluster/members: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for nodeId mismatch, got %d", resp.StatusCode)
	}
}

func postClusterMember(t *testing.T, client *http.Client, serverAddr string, body string) (int, string) {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		fmt.Sprintf("https://%s/cluster/members", serverAddr), strings.NewReader(body))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /cluster/members: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return resp.StatusCode, string(raw)
}

func TestClusterHTTP_AddMemberRejectsUnreachableJoiner(t *testing.T) {
	pki := testutil.GenTestPKI(t, []string{"1", "5"})

	serverAddr, clients, cleanup := clusterTestServer(t, pki, []string{"5"})
	defer cleanup()

	body := fmt.Sprintf(
		`{"nodeId":5,"raftAddress":"127.0.0.1:1","apiAddress":"127.0.0.1:9001","schemaVersion":%d}`,
		db.SchemaVersion(),
	)

	status, respBody := postClusterMember(t, clients["5"], serverAddr, body)

	if status != http.StatusBadGateway {
		t.Fatalf("expected 502 for an unreachable joiner, got %d (body: %s)", status, respBody)
	}

	if !strings.Contains(respBody, "Cannot reach node") {
		t.Errorf("body = %s, want the unreachable-joiner message", respBody)
	}
}

func TestClusterHTTP_AddMemberRejectsAddressHeldByAnotherNode(t *testing.T) {
	pki := testutil.GenTestPKI(t, []string{"1", "5", "6"})

	serverAddr, clients, cleanup := clusterTestServer(t, pki, []string{"5", "6"})
	defer cleanup()

	joinerLn := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           "5",
		Pin:              pki.PinFunc(),

		Leaf: pki.LeafFunc("5"),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := joinerLn.Start(ctx); err != nil {
		t.Fatalf("start joiner listener: %v", err)
	}

	defer joinerLn.Stop()

	body := fmt.Sprintf(
		`{"nodeId":5,"raftAddress":%q,"apiAddress":"127.0.0.1:9001","schemaVersion":%d,"suffrage":"nonvoter"}`,
		joinerLn.BoundAddress(), db.SchemaVersion(),
	)

	status, respBody := postClusterMember(t, clients["5"], serverAddr, body)
	if status < 200 || status >= 300 {
		t.Fatalf("registering node 5: got %d (body: %s)", status, respBody)
	}

	body = fmt.Sprintf(
		`{"nodeId":6,"raftAddress":%q,"apiAddress":"127.0.0.1:9002","schemaVersion":%d}`,
		joinerLn.BoundAddress(), db.SchemaVersion(),
	)

	status, respBody = postClusterMember(t, clients["6"], serverAddr, body)

	if status != http.StatusConflict {
		t.Fatalf("expected 409 for an address held by another node, got %d (body: %s)", status, respBody)
	}

	if !strings.Contains(respBody, "already in use by node 5") {
		t.Errorf("body = %s, want it to name node 5", respBody)
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
	pki := testutil.GenTestPKI(t, []string{"1", "5"})

	serverAddr, clients, cleanup := clusterTestServer(t, pki, []string{"5"})
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

	resp, err := clients["5"].Do(req)
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
