// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pkiagent_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/pkiagent"
	"github.com/ellanetworks/core/internal/pki"
)

// newAgent returns an Agent with an initial self-signed cluster
// cert generated for nodeID/clusterID.
func newAgent(t *testing.T, nodeID int, clusterID string) *pkiagent.Agent {
	t.Helper()

	a := pkiagent.NewAgent(nodeID, clusterID, t.TempDir())
	if err := a.GenerateAndPersist(); err != nil {
		t.Fatalf("agent %d generate-and-persist: %v", nodeID, err)
	}

	return a
}

// startListener builds a listener bound to a free ephemeral port, runs setup
// (e.g. Register, which must precede Start) if given, and starts it, retrying on
// a fresh port if another test grabbed the chosen one between selection and bind
// (the probe-then-bind window is inherently racy). Stop is registered as cleanup.
func startListener(ctx context.Context, t *testing.T, a *pkiagent.Agent, pinFn listener.PinFunc, setup func(*listener.Listener)) string {
	t.Helper()

	for attempt := 0; attempt < 20; attempt++ {
		lc := net.ListenConfig{}

		probe, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("free port: %v", err)
		}

		addr := probe.Addr().String()
		_ = probe.Close()

		ln := listener.New(listener.Config{
			BindAddress:      addr,
			AdvertiseAddress: addr,
			NodeID:           a.NodeID,
			Pin:              pinFn,
			Leaf:             func() *tls.Certificate { return a.Leaf() },
		})

		if setup != nil {
			setup(ln)
		}

		err = ln.Start(ctx)
		if err == nil {
			t.Cleanup(ln.Stop)
			return addr
		}

		if errors.Is(err, syscall.EADDRINUSE) {
			continue
		}

		t.Fatalf("start listener: %v", err)
	}

	t.Fatal("could not bind a free ephemeral port after 20 attempts")

	return ""
}

// alwaysFailRegisterHandler reads one HTTP request and writes a
// 500 response. The body is ignored.
func alwaysFailRegisterHandler() listener.ConnHandler {
	return func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

		br := bufio.NewReader(conn)
		if _, err := http.ReadRequest(br); err != nil {
			return
		}

		body := []byte("simulated leader failure")
		resp := &http.Response{
			StatusCode:    http.StatusInternalServerError,
			ProtoMajor:    1,
			ProtoMinor:    1,
			Header:        http.Header{"Content-Type": []string{"text/plain"}},
			Body:          io.NopCloser(bytes.NewReader(body)),
			ContentLength: int64(len(body)),
		}

		_ = resp.Write(conn)
	}
}

func TestAgent_JoinFlow_ReusesIdentityAcrossRetries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	leader := newAgent(t, 1, "join-cluster")

	pinFn := func(fp string) listener.PinResult {
		return listener.PinResult{Found: fp == pki.Fingerprint(leader.Leaf().Leaf), NodeID: leader.NodeID}
	}

	leaderAddr := startListener(ctx, t, leader, pinFn, func(ln *listener.Listener) {
		ln.Register(listener.ALPNPKIBootstrap, alwaysFailRegisterHandler())
	})

	joiner := pkiagent.NewAgent(2, "", t.TempDir())
	token := mintTestJoinToken(t, 2, "join-cluster", pki.Fingerprint(leader.Leaf().Leaf))

	joinCert := filepath.Join(joiner.DataDir, "cluster-tls", "join.crt")

	if err := joiner.JoinFlow(ctx, leaderAddr, token); err == nil {
		t.Fatal("JoinFlow should have failed; leader returns 500")
	}

	first, err := os.ReadFile(joinCert)
	if err != nil {
		t.Fatalf("join.crt must be persisted before the first POST: %v", err)
	}

	if err := joiner.JoinFlow(ctx, leaderAddr, token); err == nil {
		t.Fatal("second JoinFlow should have failed too")
	}

	second, err := os.ReadFile(joinCert)
	if err != nil {
		t.Fatalf("read join.crt after retry: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Error("retry generated a new identity; the leader's pin would no longer match")
	}

	if joiner.HaveLeafOnDisk() {
		t.Error("a failed join must not install a live leaf")
	}
}

func mintTestJoinToken(t *testing.T, nodeID int, clusterID, leaderPin string) string {
	t.Helper()

	key := bytes.Repeat([]byte{0xAB}, 32)
	now := time.Now()

	token, err := pki.MintJoinToken(key, pki.JoinClaims{
		TokenID:       "test-token",
		NodeID:        nodeID,
		IssuedAt:      now.Unix(),
		ExpiresAt:     now.Add(time.Hour).Unix(),
		LeaderCertPin: leaderPin,
		ClusterID:     clusterID,
		ClusterPins:   []string{leaderPin},
	})
	if err != nil {
		t.Fatalf("mint join token: %v", err)
	}

	return token
}

func TestAgent_JoinFlow_DiscardsIdentityFromAnotherCluster(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	leader := newAgent(t, 1, "cluster-b")

	pinFn := func(fp string) listener.PinResult {
		return listener.PinResult{Found: fp == pki.Fingerprint(leader.Leaf().Leaf), NodeID: leader.NodeID}
	}

	leaderAddr := startListener(ctx, t, leader, pinFn, func(ln *listener.Listener) {
		ln.Register(listener.ALPNPKIBootstrap, alwaysFailRegisterHandler())
	})

	joiner := pkiagent.NewAgent(2, "", t.TempDir())
	joinCert := filepath.Join(joiner.DataDir, "cluster-tls", "join.crt")

	tokenA := mintTestJoinToken(t, 2, "cluster-a", pki.Fingerprint(leader.Leaf().Leaf))
	if err := joiner.JoinFlow(ctx, leaderAddr, tokenA); err == nil {
		t.Fatal("JoinFlow should have failed; leader returns 500")
	}

	stale, err := os.ReadFile(joinCert)
	if err != nil {
		t.Fatalf("read join.crt: %v", err)
	}

	joiner.ClusterID = ""

	tokenB := mintTestJoinToken(t, 2, "cluster-b", pki.Fingerprint(leader.Leaf().Leaf))
	if err := joiner.JoinFlow(ctx, leaderAddr, tokenB); err == nil {
		t.Fatal("JoinFlow should have failed; leader returns 500")
	}

	fresh, err := os.ReadFile(joinCert)
	if err != nil {
		t.Fatalf("read join.crt after cluster change: %v", err)
	}

	if bytes.Equal(stale, fresh) {
		t.Fatal("kept an identity minted for another cluster; every retry would be rejected")
	}

	cert, err := pki.ParseCertPEM(fresh)
	if err != nil {
		t.Fatalf("parse regenerated join.crt: %v", err)
	}

	clusterID, _, err := pki.IdentityFromCert(cert)
	if err != nil || clusterID != "cluster-b" {
		t.Fatalf("regenerated cert is for %q, want cluster-b (err=%v)", clusterID, err)
	}
}
