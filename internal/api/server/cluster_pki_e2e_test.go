// Copyright 2026 Ella Networks

package server_test

import (
	"context"
	"crypto/tls"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/pkiagent"
	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/pki"
)

// TestClusterPKI_JoinFlowEndToEnd wires the leader-side issuer and
// bootstrap ALPN handler to a real listener, runs Agent.JoinFlow
// from a fresh joiner, and asserts that:
//
//  1. the joiner's pin row lands in cluster_node_certs;
//  2. the joiner can subsequently complete a regular mTLS
//     handshake against the leader (proving the pin was honoured
//     by the verifier).
func TestClusterPKI_JoinFlowEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const clusterID = "e2e-cluster"

	leaderDB, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "leader.db"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = leaderDB.Close() })

	if err := leaderDB.UpdateOperatorClusterID(ctx, clusterID); err != nil {
		t.Fatalf("set clusterID: %v", err)
	}

	// Leader's own cert generated and pre-pinned. MintJoinToken
	// embeds this fingerprint so the joiner pins the bootstrap TLS
	// handshake.
	leaderAgent := pkiagent.NewAgent(1, clusterID, t.TempDir())
	if err := leaderAgent.GenerateAndPersist(); err != nil {
		t.Fatalf("leader generate: %v", err)
	}

	leaderLeaf := leaderAgent.Leaf().Leaf
	if err := leaderDB.UpsertClusterNodeCert(ctx, &db.ClusterNodeCert{
		NodeID:      1,
		Fingerprint: pki.Fingerprint(leaderLeaf),
		CertPEM:     string(pki.EncodeCertPEM(leaderLeaf)),
		AddedAt:     time.Now().Unix(),
	}); err != nil {
		t.Fatalf("preregister leader pin: %v", err)
	}

	issuer := pkiissuer.New(leaderDB)
	if err := issuer.Bootstrap(ctx); err != nil {
		t.Fatalf("issuer bootstrap: %v", err)
	}

	// Pin lookup reads through to the DB so a freshly registered
	// joiner is recognised on the next handshake without manual
	// cache management.
	pinFn := func(fp string) listener.PinResult {
		row, err := leaderDB.GetClusterNodeCertByFingerprint(ctx, fp)
		if err != nil {
			return listener.PinResult{}
		}

		return listener.PinResult{Found: true, NodeID: row.NodeID}
	}

	leaderLn, leaderAddr := newE2EListener(t, leaderAgent, pinFn)
	server.RegisterBootstrapALPN(leaderLn, issuer)

	// Empty handler so the post-pin mTLS dial completes cleanly
	// rather than racing the dispatcher's "unknown ALPN" close.
	leaderLn.Register(listener.ALPNHTTP, func(conn net.Conn) {
		_ = conn.Close()
	})

	if err := leaderLn.Start(ctx); err != nil {
		t.Fatalf("start leader listener: %v", err)
	}

	t.Cleanup(leaderLn.Stop)

	// Mint a token for nodeID 2 (the joiner). Leader is nodeID 1.
	token, err := issuer.MintJoinToken(ctx, 2, 5*time.Minute, 1)
	if err != nil {
		t.Fatalf("mint join token: %v", err)
	}

	// Fresh joiner agent. ClusterID intentionally left empty —
	// JoinFlow is responsible for pulling it from the token.
	joiner := pkiagent.NewAgent(2, "", t.TempDir())

	if err := joiner.JoinFlow(ctx, leaderAddr, token); err != nil {
		t.Fatalf("join flow: %v", err)
	}

	if joiner.ClusterID != clusterID {
		t.Fatalf("joiner did not pick up clusterID from token: got %q", joiner.ClusterID)
	}

	joinerLeaf := joiner.Leaf()
	if joinerLeaf == nil || joinerLeaf.Leaf == nil {
		t.Fatal("joiner has no live cert after JoinFlow")
	}

	row, err := leaderDB.GetClusterNodeCertByFingerprint(ctx, pki.Fingerprint(joinerLeaf.Leaf))
	if err != nil {
		t.Fatalf("registered pin not found: %v", err)
	}

	if row.NodeID != 2 {
		t.Fatalf("registered pin has nodeID %d, want 2", row.NodeID)
	}

	// Replay protection: a second JoinFlow with the same token
	// must fail (single-use).
	replay := pkiagent.NewAgent(2, "", t.TempDir())
	if err := replay.JoinFlow(ctx, leaderAddr, token); err == nil {
		t.Fatal("second JoinFlow with the same token should fail")
	}

	// Follow-up mTLS handshake from the joiner to the leader on
	// ALPNHTTP. This verifies the leader's verifier accepted the
	// freshly registered pin.
	joinerLn, _ := newE2EListener(t, joiner, pinFn)

	if err := joinerLn.Start(ctx); err != nil {
		t.Fatalf("start joiner listener: %v", err)
	}

	t.Cleanup(joinerLn.Stop)

	conn, err := joinerLn.Dial(ctx, leaderAddr, 1, listener.ALPNHTTP, 5*time.Second)
	if err != nil {
		t.Fatalf("post-join mTLS dial failed: %v", err)
	}

	_ = conn.Close()

	// Sanity: a reused token attempt should not somehow have
	// left a stale row.
	if _, err := leaderDB.GetJoinToken(ctx, extractTokenID(t, token)); err != nil {
		t.Fatalf("token row missing: %v", err)
	}
}

func newE2EListener(t *testing.T, a *pkiagent.Agent, pinFn listener.PinFunc) (*listener.Listener, string) {
	t.Helper()

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

	return ln, addr
}

func extractTokenID(t *testing.T, tokenStr string) string {
	t.Helper()

	claims, err := pki.ExtractClaimsUnverified(tokenStr)
	if err != nil {
		t.Fatalf("extract token claims: %v", err)
	}

	return claims.TokenID
}
