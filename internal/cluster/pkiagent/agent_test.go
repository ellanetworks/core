// Copyright 2026 Ella Networks

package pkiagent_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/pkiagent"
	"github.com/ellanetworks/core/internal/pki"
)

// TestAgent_Rotate_RollbackOnPOSTFailure asserts the rotation
// invariant: when /cluster/pki/register returns a non-2xx, the
// agent's live cert and on-disk material remain unchanged so the
// next handshake still presents the previously-pinned cert.
func TestAgent_Rotate_RollbackOnPOSTFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	joiner := newAgent(t, 2, "rollback-cluster")
	leader := newAgent(t, 1, "rollback-cluster")

	pins := map[string]int{
		pki.Fingerprint(joiner.Leaf().Leaf): joiner.NodeID,
		pki.Fingerprint(leader.Leaf().Leaf): leader.NodeID,
	}

	pinFn := func(fp string) listener.PinResult {
		nid, ok := pins[fp]
		return listener.PinResult{Found: ok, NodeID: nid}
	}

	joinerLn, _ := newListener(t, joiner, pinFn)
	leaderLn, leaderAddr := newListener(t, leader, pinFn)

	// Leader rejects every register attempt with 500. Rotate must
	// roll back rather than commit a cert no peer can verify.
	leaderLn.Register(listener.ALPNHTTP, alwaysFailRegisterHandler())

	if err := joinerLn.Start(ctx); err != nil {
		t.Fatalf("start joiner listener: %v", err)
	}

	defer joinerLn.Stop()

	if err := leaderLn.Start(ctx); err != nil {
		t.Fatalf("start leader listener: %v", err)
	}

	defer leaderLn.Stop()

	tmpDir := joiner.DataDir
	certPath := filepath.Join(tmpDir, "cluster-tls", "leaf.crt")
	keyPath := filepath.Join(tmpDir, "cluster-tls", "leaf.key")

	beforeFP := pki.Fingerprint(joiner.Leaf().Leaf)

	beforeCert, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("read leaf.crt: %v", err)
	}

	beforeKey, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read leaf.key: %v", err)
	}

	if err := joiner.Rotate(ctx, joinerLn, leaderAddr, leader.NodeID); err == nil {
		t.Fatal("Rotate should have failed; leader returns 500")
	}

	if got := pki.Fingerprint(joiner.Leaf().Leaf); got != beforeFP {
		t.Errorf("Leaf fingerprint changed after failed rotate: was %s, now %s", beforeFP, got)
	}

	afterCert, _ := os.ReadFile(certPath)
	if !bytes.Equal(beforeCert, afterCert) {
		t.Error("leaf.crt was modified after a failed rotation")
	}

	afterKey, _ := os.ReadFile(keyPath)
	if !bytes.Equal(beforeKey, afterKey) {
		t.Error("leaf.key was modified after a failed rotation")
	}
}

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

func newListener(t *testing.T, a *pkiagent.Agent, pinFn listener.PinFunc) (*listener.Listener, string) {
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
