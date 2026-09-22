// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/hashicorp/raft"
)

// testConnListener is a channel-backed net.Listener for feeding accepted
// connections into an http.Server in tests. Mirrors the connListener in
// cluster_http.go but lives in the test file to avoid cross-package deps.
type testConnListener struct {
	ch     chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newTestConnListener() *testConnListener {
	return &testConnListener{
		ch:     make(chan net.Conn, 16),
		closed: make(chan struct{}),
	}
}

func (l *testConnListener) Accept() (net.Conn, error) {
	select {
	case conn, ok := <-l.ch:
		if !ok {
			return nil, net.ErrClosed
		}

		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *testConnListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return nil
}

func (l *testConnListener) Addr() net.Addr {
	return &net.TCPAddr{}
}

// testOpaqueConn hides *tls.Conn from http.Server so it does not
// inspect the ALPN protocol and drop the connection.
type testOpaqueConn struct{ net.Conn }

func (l *testConnListener) enqueue(conn net.Conn) {
	select {
	case l.ch <- &testOpaqueConn{conn}:
	case <-l.closed:
		_ = conn.Close()
	}
}

func discoveryFreePort(t *testing.T) int {
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

// startTestClusterHTTP registers an ALPN HTTP handler on the given listener
// and starts an http.Server that serves the provided handler. Returns a
// cleanup function.
func startTestClusterHTTP(t *testing.T, ln *listener.Listener, handler http.Handler) {
	t.Helper()

	cl := newTestConnListener()
	srv := &http.Server{Handler: handler}

	ln.Register(listener.ALPNHTTP, cl.enqueue)

	go func() { _ = srv.Serve(cl) }()

	t.Cleanup(func() {
		_ = cl.Close()
		_ = srv.Close()
	})
}

func newProbePeerHarness(t *testing.T, handler http.Handler) (*Manager, string) {
	t.Helper()

	pki := testutil.GenTestPKI(t, []string{"1", "2"})

	serverAddr := fmt.Sprintf("127.0.0.1:%d", discoveryFreePort(t))

	serverLn := listener.New(listener.Config{
		BindAddress:      serverAddr,
		AdvertiseAddress: serverAddr,
		NodeID:           "1",
		Pin:              pki.PinFunc(),
		Leaf:             pki.LeafFunc("1"),
	})

	startTestClusterHTTP(t, serverLn, handler)

	if err := serverLn.Start(t.Context()); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	t.Cleanup(serverLn.Stop)

	clientLn := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           "2",
		Pin:              pki.PinFunc(),
		Leaf:             pki.LeafFunc("2"),
	})

	return &Manager{clusterListener: clientLn}, serverAddr
}

func statusHandler(cluster *statusClusterBlock) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(statusResponse{Result: statusResult{Cluster: cluster}})
	})
}

func TestProbePeer(t *testing.T) {
	tests := []struct {
		name          string
		handler       http.Handler
		wantState     peerState
		wantNodeID    string
		wantClusterID string
		wantSchema    int
	}{
		{
			name: "leader",
			handler: statusHandler(&statusClusterBlock{
				Role: "Leader", NodeID: "1", ClusterID: "cluster-1", SchemaVersion: 9,
			}),
			wantState:     peerFormed,
			wantNodeID:    "1",
			wantClusterID: "cluster-1",
			wantSchema:    9,
		},
		{
			name: "follower",
			handler: statusHandler(&statusClusterBlock{
				Role: "Follower", NodeID: "2", ClusterID: "cluster-1", SchemaVersion: 9,
			}),
			wantState:     peerFormed,
			wantNodeID:    "2",
			wantClusterID: "cluster-1",
			wantSchema:    9,
		},
		{
			name:       "forming",
			handler:    statusHandler(&statusClusterBlock{Role: "Follower", NodeID: "3"}),
			wantState:  peerForming,
			wantNodeID: "3",
		},
		{
			name: "unavailable",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			}),
			wantState: peerUnreachable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, serverAddr := newProbePeerHarness(t, tt.handler)

			probe := m.probePeer(t.Context(), serverAddr)

			if probe.state != tt.wantState {
				t.Errorf("state = %d, want %d", probe.state, tt.wantState)
			}

			if probe.nodeID != tt.wantNodeID {
				t.Errorf("nodeID = %s, want %s", probe.nodeID, tt.wantNodeID)
			}

			if probe.clusterID != tt.wantClusterID {
				t.Errorf("clusterID = %q, want %q", probe.clusterID, tt.wantClusterID)
			}

			if probe.schemaVersion != tt.wantSchema {
				t.Errorf("schema = %d, want %d", probe.schemaVersion, tt.wantSchema)
			}
		})
	}
}

// TestDiscoveryTick_DuplicateNodeIDFails verifies that discoveryTick fails
// hard when a reachable peer advertises the same node-id as this node.
// Warning and continuing would risk silent split-brain at bootstrap or a
// join request that clobbers an existing cluster member.
func TestDiscoveryTick_DuplicateNodeIDFails(t *testing.T) {
	cases := []struct {
		name string
		role string // "" means forming (no clusterId), "Leader"/"Follower" means formed
	}{
		{name: "forming_peer", role: ""},
		{name: "formed_peer", role: "Leader"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pki := testutil.GenTestPKI(t, []string{"1", "2"})

			serverPort := discoveryFreePort(t)
			serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)

			serverLn := listener.New(listener.Config{
				BindAddress:      serverAddr,
				AdvertiseAddress: serverAddr,
				NodeID:           "1",
				Pin:              pki.PinFunc(),

				Leaf: pki.LeafFunc("1"),
			})

			cluster := &statusClusterBlock{
				Role:          tc.role,
				NodeID:        "2", // same as probing node
				SchemaVersion: 9,
			}

			if tc.role != "" {
				cluster.ClusterID = "cluster-1"
			}

			startTestClusterHTTP(t, serverLn, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(statusResponse{
					Result: statusResult{Cluster: cluster},
				})
			}))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := serverLn.Start(ctx); err != nil {
				t.Fatalf("start listener: %v", err)
			}

			defer serverLn.Stop()

			clientLn := listener.New(listener.Config{
				BindAddress:      "127.0.0.1:0",
				AdvertiseAddress: "127.0.0.1:0",
				NodeID:           "2",
				Pin:              pki.PinFunc(),

				Leaf: pki.LeafFunc("2"),
			})

			m := &Manager{
				raftID:          "2",
				clusterListener: clientLn,
				config: ClusterConfig{
					Peers:            []string{serverAddr},
					AdvertiseAddress: "127.0.0.1:9999",
					HasJoinToken:     true,
					SchemaVersion:    9,
				},
			}

			joined, err := m.discoveryTick(ctx)
			if err == nil {
				t.Fatalf("expected error on duplicate node-id, got nil (joined=%v)", joined)
			}

			if joined {
				t.Fatalf("joined should be false when duplicate node-id is detected")
			}
		})
	}
}

func TestDiscoveryTick_JoinsThroughTheLeader(t *testing.T) {
	testPKI := testutil.GenTestPKI(t, []string{"1", "2", "3"})

	leaderAddr := fmt.Sprintf("127.0.0.1:%d", discoveryFreePort(t))
	seedAddr := fmt.Sprintf("127.0.0.1:%d", discoveryFreePort(t))

	var mu sync.Mutex

	var joinedAt string

	statusBlock := func(role, nodeID string) *statusClusterBlock {
		return &statusClusterBlock{Role: role, NodeID: pki.NodeID(nodeID), ClusterID: "cluster-1", SchemaVersion: 9}
	}

	handlerFor := func(block *statusClusterBlock, members http.HandlerFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/cluster/status":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(statusResponse{Result: statusResult{Cluster: block}})
			case "/cluster/members":
				members(w, r)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})
	}

	serve := func(nodeID, addr string, h http.Handler) {
		ln := listener.New(listener.Config{
			BindAddress:      addr,
			AdvertiseAddress: addr,
			NodeID:           nodeID,
			Pin:              testPKI.PinFunc(),
			Leaf:             testPKI.LeafFunc(nodeID),
		})

		startTestClusterHTTP(t, ln, h)

		if err := ln.Start(t.Context()); err != nil {
			t.Fatalf("start listener %s: %v", nodeID, err)
		}

		t.Cleanup(ln.Stop)
	}

	serve("1", leaderAddr, handlerFor(statusBlock("Leader", "1"), func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		joinedAt = "leader"
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
	}))

	serve("2", seedAddr, handlerFor(statusBlock("Follower", "2"), func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		joinedAt = "seed"
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMisdirectedRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":         "this node is not the cluster leader",
			"leaderNodeId":  "1",
			"leaderAddress": leaderAddr,
		})
	}))

	clientLn := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           "3",
		Pin:              testPKI.PinFunc(),
		Leaf:             testPKI.LeafFunc("3"),
	})

	_, transport := raft.NewInmemTransport("127.0.0.1:9999")

	m := &Manager{
		raftID:          "3",
		clusterListener: clientLn,
		transport:       transport,
		config: ClusterConfig{
			Peers:            []string{seedAddr},
			AdvertiseAddress: "127.0.0.1:9999",
			HasJoinToken:     true,
			SchemaVersion:    9,
		},
	}

	joined, err := m.discoveryTick(t.Context())
	if err != nil {
		t.Fatalf("discoveryTick: %v", err)
	}

	if !joined {
		t.Fatal("expected the joiner to report success")
	}

	mu.Lock()
	defer mu.Unlock()

	if joinedAt != "leader" {
		t.Fatalf("join completed at %q, want the leader", joinedAt)
	}
}

func TestDiscoveryTick_LeaderSchemaGuardGovernsAfterRedirect(t *testing.T) {
	testPKI := testutil.GenTestPKI(t, []string{"1", "2", "3"})

	leaderAddr := fmt.Sprintf("127.0.0.1:%d", discoveryFreePort(t))
	seedAddr := fmt.Sprintf("127.0.0.1:%d", discoveryFreePort(t))

	var mu sync.Mutex

	leaderAsked := false

	statusBlock := func(role, nodeID string, schema int) *statusClusterBlock {
		return &statusClusterBlock{
			Role:          role,
			NodeID:        pki.NodeID(nodeID),
			ClusterID:     "cluster-1",
			SchemaVersion: schema,
		}
	}

	handlerFor := func(block *statusClusterBlock, members http.HandlerFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/cluster/status":
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(statusResponse{Result: statusResult{Cluster: block}})
			case "/cluster/members":
				members(w, r)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})
	}

	serve := func(nodeID, addr string, h http.Handler) {
		ln := listener.New(listener.Config{
			BindAddress:      addr,
			AdvertiseAddress: addr,
			NodeID:           nodeID,
			Pin:              testPKI.PinFunc(),
			Leaf:             testPKI.LeafFunc(nodeID),
		})

		startTestClusterHTTP(t, ln, h)

		if err := ln.Start(t.Context()); err != nil {
			t.Fatalf("start listener %s: %v", nodeID, err)
		}

		t.Cleanup(ln.Stop)
	}

	serve("1", leaderAddr, handlerFor(statusBlock("Leader", "1", 12), func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		leaderAsked = true
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "Schema version mismatch: node has 9, cluster has 12",
		})
	}))

	serve("2", seedAddr, handlerFor(statusBlock("Follower", "2", 9), func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMisdirectedRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":         "this node is not the cluster leader",
			"leaderNodeId":  "1",
			"leaderAddress": leaderAddr,
		})
	}))

	clientLn := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           "3",
		Pin:              testPKI.PinFunc(),
		Leaf:             testPKI.LeafFunc("3"),
	})

	_, transport := raft.NewInmemTransport("127.0.0.1:9999")

	m := &Manager{
		raftID:          "3",
		clusterListener: clientLn,
		transport:       transport,
		config: ClusterConfig{
			Peers:            []string{seedAddr},
			AdvertiseAddress: "127.0.0.1:9999",
			HasJoinToken:     true,
			SchemaVersion:    9,
		},
	}

	joined, err := m.discoveryTick(t.Context())

	if joined {
		t.Fatal("joined a cluster whose leader runs a newer schema")
	}

	if err == nil {
		t.Fatal("expected the leader's rejection to surface")
	}

	if !strings.Contains(err.Error(), leaderAddr) {
		t.Errorf("error %q does not name the leader %s", err, leaderAddr)
	}

	if !strings.Contains(err.Error(), "Schema version mismatch: node has 9, cluster has 12") {
		t.Errorf("error %q does not carry the leader's schema verdict", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !leaderAsked {
		t.Fatal("the joiner never reached the leader; the seed's schema decided the outcome")
	}
}

func TestDiscoveryTick_KeepsTheMessageWhenTheRedirectNamesNoLeader(t *testing.T) {
	const legacyMessage = "This node is not the cluster leader; retry against node node-a (11111111-1111-1111-1111-111111111111) at 10.0.0.1:5002"

	for _, tc := range []struct {
		name string
		body map[string]string
	}{
		{
			name: "legacy envelope carries only prose",
			body: map[string]string{"error": legacyMessage},
		},
		{
			name: "no leader elected yet",
			body: map[string]string{"error": "this node is not the cluster leader"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testPKI := testutil.GenTestPKI(t, []string{"1", "2"})

			seedAddr := fmt.Sprintf("127.0.0.1:%d", discoveryFreePort(t))

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/cluster/status":
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(statusResponse{Result: statusResult{Cluster: &statusClusterBlock{
						Role: "Follower", NodeID: "1", ClusterID: "cluster-1", SchemaVersion: 9,
					}}})
				case "/cluster/members":
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusMisdirectedRequest)
					_ = json.NewEncoder(w).Encode(tc.body)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			})

			seedLn := listener.New(listener.Config{
				BindAddress:      seedAddr,
				AdvertiseAddress: seedAddr,
				NodeID:           "1",
				Pin:              testPKI.PinFunc(),
				Leaf:             testPKI.LeafFunc("1"),
			})

			startTestClusterHTTP(t, seedLn, handler)

			if err := seedLn.Start(t.Context()); err != nil {
				t.Fatalf("start listener: %v", err)
			}

			t.Cleanup(seedLn.Stop)

			clientLn := listener.New(listener.Config{
				BindAddress:      "127.0.0.1:0",
				AdvertiseAddress: "127.0.0.1:0",
				NodeID:           "2",
				Pin:              testPKI.PinFunc(),
				Leaf:             testPKI.LeafFunc("2"),
			})

			_, transport := raft.NewInmemTransport("127.0.0.1:9999")

			m := &Manager{
				raftID:          "2",
				clusterListener: clientLn,
				transport:       transport,
				config: ClusterConfig{
					Peers:            []string{seedAddr},
					AdvertiseAddress: "127.0.0.1:9999",
					HasJoinToken:     true,
					SchemaVersion:    9,
				},
			}

			joined, err := m.discoveryTick(t.Context())

			if joined {
				t.Fatal("joined through a peer that refused the join")
			}

			if err == nil {
				t.Fatal("expected the peer's refusal to surface")
			}

			if !strings.Contains(err.Error(), tc.body["error"]) {
				t.Errorf("error %q dropped the peer's message %q", err, tc.body["error"])
			}
		})
	}
}

func TestDiscoveryTick_SkipsPeerThatNamesThisNodeAsLeader(t *testing.T) {
	testPKI := testutil.GenTestPKI(t, []string{"1", "2"})

	seedAddr := fmt.Sprintf("127.0.0.1:%d", discoveryFreePort(t))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cluster/status":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(statusResponse{Result: statusResult{Cluster: &statusClusterBlock{
				Role: "Follower", NodeID: "1", ClusterID: "cluster-1", SchemaVersion: 9,
			}}})
		case "/cluster/members":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMisdirectedRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":         "this node is not the cluster leader",
				"leaderNodeId":  "2",
				"leaderAddress": "127.0.0.1:1",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	ln := listener.New(listener.Config{
		BindAddress:      seedAddr,
		AdvertiseAddress: seedAddr,
		NodeID:           "1",
		Pin:              testPKI.PinFunc(),
		Leaf:             testPKI.LeafFunc("1"),
	})

	startTestClusterHTTP(t, ln, handler)

	if err := ln.Start(t.Context()); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	t.Cleanup(ln.Stop)

	clientLn := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           "2",
		Pin:              testPKI.PinFunc(),
		Leaf:             testPKI.LeafFunc("2"),
	})

	_, transport := raft.NewInmemTransport("127.0.0.1:9999")

	m := &Manager{
		raftID:          "2",
		clusterListener: clientLn,
		transport:       transport,
		config: ClusterConfig{
			Peers:            []string{seedAddr},
			AdvertiseAddress: "127.0.0.1:9999",
			HasJoinToken:     true,
			SchemaVersion:    9,
		},
	}

	joined, err := m.discoveryTick(t.Context())
	if joined {
		t.Fatal("expected the joiner not to join")
	}

	if errors.Is(err, ErrDiscoveryFatal) {
		t.Fatalf("stale leader information must not be fatal, got %v", err)
	}

	if err == nil || !strings.Contains(err.Error(), "names this node") {
		t.Fatalf("err = %v, want a retryable skip naming this node", err)
	}
}
