// Copyright 2026 Ella Networks

package server_test

import (
	"context"
	"encoding/json"
	"fmt"
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
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[1].TLSCert,
	})

	dbPath := filepath.Join(t.TempDir(), "test.db")

	testDB, err := db.NewDatabase(context.Background(), dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	stopCluster := server.StartClusterHTTP(testDB, serverLn, nil)
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
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[2].TLSCert,
	})

	client := &http.Client{
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return clientLn.Dial(ctx, addr, listener.ALPNHTTP, 5*time.Second)
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
func clusterTestServer(t *testing.T, pki *testutil.PKI, serverNodeID int, peerNodeIDs []int) (serverAddr string, clients map[int]*http.Client, cleanup func()) {
	t.Helper()

	port := clusterFreePort(t)
	serverAddr = fmt.Sprintf("127.0.0.1:%d", port)

	serverLn := listener.New(listener.Config{
		BindAddress:      serverAddr,
		AdvertiseAddress: serverAddr,
		NodeID:           serverNodeID,
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[serverNodeID].TLSCert,
	})

	dbPath := filepath.Join(t.TempDir(), "test.db")

	testDB, err := db.NewDatabase(context.Background(), dbPath, ellaraft.ClusterConfig{})
	if err != nil {
		t.Fatalf("create test db: %v", err)
	}

	stopCluster := server.StartClusterHTTP(testDB, serverLn, nil)

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
			CAPool:           pki.CAPool,
			LeafCert:         pki.Nodes[id].TLSCert,
		})

		clients[id] = &http.Client{
			Transport: &http.Transport{
				DialTLSContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
					return clientLn.Dial(ctx, addr, listener.ALPNHTTP, 5*time.Second)
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

	serverAddr, clients, cleanup := clusterTestServer(t, pki, 1, []int{5})
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
