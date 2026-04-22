// Copyright 2026 Ella Networks

package pkiagent_test

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/listener/testutil"
	"github.com/ellanetworks/core/internal/cluster/pkiagent"
	"github.com/ellanetworks/core/internal/pki"
)

func jsonUnmarshal(data []byte, v any) error { return json.Unmarshal(data, v) }

func TestAgent_StoreAndLoad(t *testing.T) {
	pkiHelper := testutil.GenTestPKI(t, []int{1})

	dataDir := t.TempDir()
	agent := pkiagent.NewAgent(1, "test-cluster", dataDir)

	leaf := pkiHelper.Nodes[1]

	bundle := pkiHelper.Bundle()

	var bundlePEM []byte

	for _, r := range bundle.Roots {
		bundlePEM = append(bundlePEM, pki.EncodeCertPEM(r)...)
	}

	for _, i := range bundle.Intermediates {
		bundlePEM = append(bundlePEM, pki.EncodeCertPEM(i)...)
	}

	if err := agent.StoreLeaf(leaf.CertPEM, leaf.KeyPEM, bundlePEM); err != nil {
		t.Fatalf("StoreLeaf: %v", err)
	}

	if agent.Leaf() == nil {
		t.Fatal("Leaf() should return current cert after StoreLeaf")
	}

	// Reload from disk.
	agent2 := pkiagent.NewAgent(1, "test-cluster", dataDir)

	if !agent2.HaveLeafOnDisk() {
		t.Fatal("HaveLeafOnDisk should be true after StoreLeaf")
	}

	if err := agent2.Load(bundlePEM); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if agent2.Leaf() == nil {
		t.Fatal("Leaf() nil after Load()")
	}

	if agent2.Leaf().Leaf == nil {
		t.Fatal("parsed Leaf on tls.Certificate must be populated")
	}

	if agent2.Leaf().Leaf.Subject.CommonName != "ella-node-1" {
		t.Fatalf("CN = %q", agent2.Leaf().Leaf.Subject.CommonName)
	}
}

func TestAgent_RenewSchedule(t *testing.T) {
	pkiHelper := testutil.GenTestPKI(t, []int{1})

	dataDir := t.TempDir()
	agent := pkiagent.NewAgent(1, "test-cluster", dataDir)

	leaf := pkiHelper.Nodes[1]

	var bundlePEM []byte
	for _, r := range pkiHelper.Bundle().Roots {
		bundlePEM = append(bundlePEM, pki.EncodeCertPEM(r)...)
	}

	if err := agent.StoreLeaf(leaf.CertPEM, leaf.KeyPEM, bundlePEM); err != nil {
		t.Fatal(err)
	}

	now := time.Now()

	soft, hard := agent.RenewSchedule(now)
	if soft.IsZero() || hard.IsZero() {
		t.Fatal("RenewSchedule must return real times when leaf is loaded")
	}

	if !soft.Before(hard) {
		t.Fatal("soft must precede hard")
	}

	// PickRenewAt must fall within [soft, hard] for values of now before soft.
	beforeSoft := soft.Add(-10 * time.Second)
	pick := agent.PickRenewAt(beforeSoft)

	if pick.Before(soft) || pick.After(hard) {
		t.Fatalf("pick %s outside [%s, %s]", pick, soft, hard)
	}
}

// TestAgent_JoinFlow_AgainstRealListener runs the agent's JoinFlow
// against a real cluster listener configured with a test PKI. The
// listener answers on the bootstrap ALPN without requiring a client
// cert, so the join path is exercised end-to-end without httptest
// interference.
func TestAgent_JoinFlow_AgainstRealListener(t *testing.T) {
	p := testutil.GenTestPKI(t, []int{1})

	bundle := p.Bundle()
	leaf1 := p.Nodes[1].TLSCert
	// Extend the leaf's chain so the server sends the full root down
	// to the pinning client (mirrors production where the leader's
	// listener is configured to include the root).
	leaf1Full := leaf1
	leaf1Full.Certificate = [][]byte{leaf1.Certificate[0], p.Intermediate.Raw, p.Root.Raw}

	port := freePort(t)
	addr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: port}

	ln := listener.New(listener.Config{
		BindAddress:      addr.String(),
		AdvertiseAddress: addr.String(),
		NodeID:           1,
		TrustBundle:      func() *pki.TrustBundle { return bundle },
		Leaf:             func() *tls.Certificate { return &leaf1Full },
		Revoked:          func(*big.Int) bool { return false },
	})

	t.Cleanup(func() { ln.Stop() })

	// Minimal bootstrap handler: parse the incoming CSR, sign a leaf
	// for it using the test PKI's intermediate, return the signed
	// leaf + bundle. Mirrors the real /cluster/pki/issue handler.
	ln.Register(listener.ALPNPKIBootstrap, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		br := bufio.NewReader(conn)

		req, err := http.ReadRequest(br)
		if err != nil {
			t.Logf("handler ReadRequest: %v", err)
			return
		}

		bodyBytes, _ := io.ReadAll(req.Body)

		var issueReq pkiagent.IssueRequest
		if err := jsonUnmarshal(bodyBytes, &issueReq); err != nil {
			t.Logf("handler decode: %v", err)
			return
		}

		csr, err := pki.ParseCSRPEM([]byte(issueReq.CSRPEM))
		if err != nil {
			t.Logf("handler parse csr: %v", err)
			return
		}

		issuer := pki.NewIssuer(p.Intermediate, p.IntermediateKey, p.ClusterID)

		leafPEM, err := issuer.SignLeaf(csr, issueReq.NodeID, 99, time.Hour)
		if err != nil {
			t.Logf("handler sign: %v", err)
			return
		}

		var bundlePEM []byte
		for _, root := range bundle.Roots {
			bundlePEM = append(bundlePEM, pki.EncodeCertPEM(root)...)
		}

		body := `{"leafPEM":` + quoteJSON(string(leafPEM)) + `,"bundlePEM":` + quoteJSON(string(bundlePEM)) + `}`

		resp := &http.Response{
			StatusCode:    200,
			ProtoMajor:    1,
			ProtoMinor:    1,
			ContentLength: int64(len(body)),
			Body:          io.NopCloser(strings.NewReader(body)),
			Header:        http.Header{"Content-Type": []string{"application/json"}},
		}

		_ = resp.Write(conn)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := ln.Start(ctx); err != nil {
		t.Fatal(err)
	}

	token := mintTestToken(t, 2, pki.Fingerprint(p.Root))

	dataDir := t.TempDir()
	agent := pkiagent.NewAgent(2, p.ClusterID, dataDir)

	if err := agent.JoinFlow(ctx, addr.String(), token); err != nil {
		t.Fatalf("JoinFlow: %v", err)
	}

	if agent.Leaf() == nil {
		t.Fatal("agent has no leaf after JoinFlow")
	}

	on, err := os.ReadFile(filepath.Join(dataDir, "cluster-tls", "leaf.crt"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := pki.ParseCertPEM(on); err != nil {
		t.Fatalf("persisted leaf not a valid cert: %v", err)
	}
}

// TestAgent_JoinFlow_WrongFingerprint exercises the pinning failure
// path against a real listener.
func TestAgent_JoinFlow_WrongFingerprint(t *testing.T) {
	p := testutil.GenTestPKI(t, []int{1})

	bundle := p.Bundle()
	leaf1 := p.Nodes[1].TLSCert
	leaf1Full := leaf1
	leaf1Full.Certificate = [][]byte{leaf1.Certificate[0], p.Intermediate.Raw, p.Root.Raw}

	port := freePort(t)

	ln := listener.New(listener.Config{
		BindAddress:      net.JoinHostPort("127.0.0.1", itoa(port)),
		AdvertiseAddress: net.JoinHostPort("127.0.0.1", itoa(port)),
		NodeID:           1,
		TrustBundle:      func() *pki.TrustBundle { return bundle },
		Leaf:             func() *tls.Certificate { return &leaf1Full },
		Revoked:          func(*big.Int) bool { return false },
	})

	t.Cleanup(func() { ln.Stop() })

	ln.Register(listener.ALPNPKIBootstrap, func(conn net.Conn) { _ = conn.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := ln.Start(ctx); err != nil {
		t.Fatal(err)
	}

	addr := net.JoinHostPort("127.0.0.1", itoa(port))

	dataDir := t.TempDir()
	agent := pkiagent.NewAgent(2, p.ClusterID, dataDir)

	// Token whose CAFingerprint points at a non-existent root — the
	// server's real cert won't match the pin, so TLS must fail.
	badToken := mintTestToken(t, 2, "sha256:0000000000000000000000000000000000000000000000000000000000000000")
	if err := agent.JoinFlow(ctx, addr, badToken); err == nil {
		t.Fatal("JoinFlow with wrong fingerprint must fail")
	}
}

// mintTestToken builds a valid join token with the given node-id and
// embedded CA fingerprint. Uses a throwaway HMAC key — the agent only
// reads the claims unverified for TLS pinning, and the test handlers
// don't validate HMAC.
func mintTestToken(t *testing.T, nodeID int, fingerprint string) string {
	t.Helper()

	key, err := pki.NewHMACKey()
	if err != nil {
		t.Fatal(err)
	}

	id, err := pki.NewTokenID()
	if err != nil {
		t.Fatal(err)
	}

	tok, err := pki.MintJoinToken(key, pki.JoinClaims{
		TokenID:       id,
		NodeID:        nodeID,
		IssuedAt:      time.Now().Unix(),
		ExpiresAt:     time.Now().Add(time.Hour).Unix(),
		CAFingerprint: fingerprint,
		ClusterID:     "test-cluster",
	})
	if err != nil {
		t.Fatal(err)
	}

	return tok
}

// ---------------------------------------------------------------------------
// test helpers
// ---------------------------------------------------------------------------

func freePort(t *testing.T) int {
	t.Helper()

	var lc net.ListenConfig

	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()

	return port
}

func itoa(n int) string {
	// Avoid importing strconv for one use.
	buf := [16]byte{}
	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	return string(buf[i:])
}

// quoteJSON returns a minimal JSON-string-escaped version of s. Good
// enough for test fixtures that never contain real JSON-edge characters.
func quoteJSON(s string) string {
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')

	for _, r := range s {
		switch r {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		default:
			out = append(out, string(r)...)
		}
	}

	out = append(out, '"')

	return string(out)
}
