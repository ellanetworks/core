// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
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

	pki := testutil.GenTestPKI(t, []int{1, 2})

	serverAddr := fmt.Sprintf("127.0.0.1:%d", discoveryFreePort(t))

	serverLn := listener.New(listener.Config{
		BindAddress:      serverAddr,
		AdvertiseAddress: serverAddr,
		NodeID:           1,
		Pin:              pki.PinFunc(),
		Leaf:             pki.LeafFunc(1),
	})

	startTestClusterHTTP(t, serverLn, handler)

	if err := serverLn.Start(t.Context()); err != nil {
		t.Fatalf("start listener: %v", err)
	}

	t.Cleanup(serverLn.Stop)

	clientLn := listener.New(listener.Config{
		BindAddress:      "127.0.0.1:0",
		AdvertiseAddress: "127.0.0.1:0",
		NodeID:           2,
		Pin:              pki.PinFunc(),
		Leaf:             pki.LeafFunc(2),
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
		wantNodeID    int
		wantClusterID string
		wantSchema    int
	}{
		{
			name: "leader",
			handler: statusHandler(&statusClusterBlock{
				Role: "Leader", NodeID: 1, ClusterID: "cluster-1", SchemaVersion: 9,
			}),
			wantState:     peerFormed,
			wantNodeID:    1,
			wantClusterID: "cluster-1",
			wantSchema:    9,
		},
		{
			name: "follower",
			handler: statusHandler(&statusClusterBlock{
				Role: "Follower", NodeID: 2, ClusterID: "cluster-1", SchemaVersion: 9,
			}),
			wantState:     peerFormed,
			wantNodeID:    2,
			wantClusterID: "cluster-1",
			wantSchema:    9,
		},
		{
			name:       "forming",
			handler:    statusHandler(&statusClusterBlock{Role: "Follower", NodeID: 3}),
			wantState:  peerForming,
			wantNodeID: 3,
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

			state, nodeID, clusterID, schema := m.probePeer(t.Context(), serverAddr)

			if state != tt.wantState {
				t.Errorf("state = %d, want %d", state, tt.wantState)
			}

			if nodeID != tt.wantNodeID {
				t.Errorf("nodeID = %d, want %d", nodeID, tt.wantNodeID)
			}

			if clusterID != tt.wantClusterID {
				t.Errorf("clusterID = %q, want %q", clusterID, tt.wantClusterID)
			}

			if schema != tt.wantSchema {
				t.Errorf("schema = %d, want %d", schema, tt.wantSchema)
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
			pki := testutil.GenTestPKI(t, []int{1, 2})

			serverPort := discoveryFreePort(t)
			serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)

			serverLn := listener.New(listener.Config{
				BindAddress:      serverAddr,
				AdvertiseAddress: serverAddr,
				NodeID:           1,
				Pin:              pki.PinFunc(),

				Leaf: pki.LeafFunc(1),
			})

			cluster := &statusClusterBlock{
				Role:          tc.role,
				NodeID:        2, // same as probing node
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
				NodeID:           2,
				Pin:              pki.PinFunc(),

				Leaf: pki.LeafFunc(2),
			})

			m := &Manager{
				nodeID:          2,
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
