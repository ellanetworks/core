// Copyright 2026 Ella Networks

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

func TestProbePeer_LeaderReturns200(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	serverPort := discoveryFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)

	serverLn := listener.New(listener.Config{
		BindAddress:      serverAddr,
		AdvertiseAddress: serverAddr,
		NodeID:           1,
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[1].TLSCert,
	})

	startTestClusterHTTP(t, serverLn, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(statusResponse{
			Result: statusResult{
				Cluster: &statusClusterBlock{
					Role:          "Leader",
					NodeID:        1,
					ClusterID:     "cluster-1",
					SchemaVersion: 9,
				},
			},
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
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[2].TLSCert,
	})

	m := &Manager{clusterListener: clientLn}

	state, nodeID, clusterID, schema := m.probePeer(ctx, serverAddr)

	if state != peerFormed {
		t.Fatalf("expected peerFormed, got %d", state)
	}

	if nodeID != 1 {
		t.Fatalf("expected nodeID=1, got %d", nodeID)
	}

	if clusterID != "cluster-1" {
		t.Fatalf("expected clusterID=cluster-1, got %s", clusterID)
	}

	if schema != 9 {
		t.Fatalf("expected schema=9, got %d", schema)
	}
}

func TestProbePeer_FollowerReturns200(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	serverPort := discoveryFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)

	serverLn := listener.New(listener.Config{
		BindAddress:      serverAddr,
		AdvertiseAddress: serverAddr,
		NodeID:           1,
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[1].TLSCert,
	})

	startTestClusterHTTP(t, serverLn, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(statusResponse{
			Result: statusResult{
				Cluster: &statusClusterBlock{
					Role:          "Follower",
					NodeID:        2,
					ClusterID:     "cluster-1",
					SchemaVersion: 9,
				},
			},
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
		NodeID:           3,
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[2].TLSCert,
	})

	m := &Manager{clusterListener: clientLn}

	state, nodeID, clusterID, schema := m.probePeer(ctx, serverAddr)

	if state != peerFormed {
		t.Fatalf("expected peerFormed, got %d", state)
	}

	if nodeID != 2 {
		t.Fatalf("expected nodeID=2, got %d", nodeID)
	}

	if clusterID != "cluster-1" {
		t.Fatalf("expected clusterID=cluster-1, got %s", clusterID)
	}

	if schema != 9 {
		t.Fatalf("expected schema=9, got %d", schema)
	}
}

func TestProbePeer_FormingNode(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	serverPort := discoveryFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)

	serverLn := listener.New(listener.Config{
		BindAddress:      serverAddr,
		AdvertiseAddress: serverAddr,
		NodeID:           1,
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[1].TLSCert,
	})

	startTestClusterHTTP(t, serverLn, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(statusResponse{
			Result: statusResult{
				Cluster: &statusClusterBlock{
					Role:   "Follower",
					NodeID: 3,
				},
			},
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
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[2].TLSCert,
	})

	m := &Manager{clusterListener: clientLn}

	state, nodeID, _, _ := m.probePeer(ctx, serverAddr)

	if state != peerForming {
		t.Fatalf("expected peerForming, got %d", state)
	}

	if nodeID != 3 {
		t.Fatalf("expected nodeID=3, got %d", nodeID)
	}
}

func TestProbePeer_503IsUnreachable(t *testing.T) {
	pki := testutil.GenTestPKI(t, []int{1, 2})

	serverPort := discoveryFreePort(t)
	serverAddr := fmt.Sprintf("127.0.0.1:%d", serverPort)

	serverLn := listener.New(listener.Config{
		BindAddress:      serverAddr,
		AdvertiseAddress: serverAddr,
		NodeID:           1,
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[1].TLSCert,
	})

	startTestClusterHTTP(t, serverLn, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
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
		CAPool:           pki.CAPool,
		LeafCert:         pki.Nodes[2].TLSCert,
	})

	m := &Manager{clusterListener: clientLn}

	state, nodeID, clusterID, schema := m.probePeer(ctx, serverAddr)

	if state != peerUnreachable {
		t.Fatalf("expected peerUnreachable, got %d", state)
	}

	if nodeID != 0 {
		t.Fatalf("expected nodeID=0, got %d", nodeID)
	}

	if clusterID != "" {
		t.Fatalf("expected empty clusterID, got %s", clusterID)
	}

	if schema != 0 {
		t.Fatalf("expected schema=0, got %d", schema)
	}
}
