// Copyright 2026 Ella Networks

// Package pkiagent runs on every cluster node. It owns the local leaf
// key, fetches a leaf at first boot (via the join flow on a joining
// node, or via the issuer service on the first node), persists the
// leaf to disk, and renews on a jittered schedule.
package pkiagent

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/pki"
)

// Filenames under <dataDir>/cluster-tls/. All three are per-node
// caches: leaf.crt and bundle.crt carry public material reassemble-able
// from the replicated DB; leaf.key is the per-node private key that
// must not be replicated.
const (
	leafCertFile   = "leaf.crt"
	leafKeyFile    = "leaf.key"
	bundleCertFile = "bundle.crt"
)

// Agent manages the local node's cluster leaf.
type Agent struct {
	NodeID    int
	ClusterID string
	DataDir   string

	current atomic.Pointer[tls.Certificate]
}

// NewAgent returns an unloaded agent. Callers should invoke Load,
// JoinFlow, or the issuer's self-issue path before relying on Leaf.
func NewAgent(nodeID int, clusterID, dataDir string) *Agent {
	return &Agent{
		NodeID:    nodeID,
		ClusterID: clusterID,
		DataDir:   dataDir,
	}
}

// Leaf is a listener.Config.Leaf accessor.
func (a *Agent) Leaf() *tls.Certificate {
	return a.current.Load()
}

// HaveLeafOnDisk reports whether leaf.crt and leaf.key both exist.
func (a *Agent) HaveLeafOnDisk() bool {
	certPath := a.path(leafCertFile)
	keyPath := a.path(leafKeyFile)

	if _, err := os.Stat(certPath); err != nil {
		return false
	}

	if _, err := os.Stat(keyPath); err != nil {
		return false
	}

	return true
}

// Load reads leaf.crt, leaf.key, and bundle.crt from disk and installs
// the in-memory tls.Certificate used by the listener. A missing
// bundle.crt is tolerated (the resulting certificate carries only the
// leaf) so first-boot listener startup can complete before the bundle
// has been written.
func (a *Agent) Load() error {
	certPEM, err := os.ReadFile(a.path(leafCertFile)) // #nosec: G304 -- under dataDir
	if err != nil {
		return fmt.Errorf("read leaf.crt: %w", err)
	}

	keyPEM, err := os.ReadFile(a.path(leafKeyFile)) // #nosec: G304
	if err != nil {
		return fmt.Errorf("read leaf.key: %w", err)
	}

	chainPEM := certPEM

	if bundlePEM, err := os.ReadFile(a.path(bundleCertFile)); err == nil { // #nosec: G304
		chainPEM = append(append([]byte{}, certPEM...), bundlePEM...)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read bundle.crt: %w", err)
	}

	c, err := tls.X509KeyPair(chainPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("load leaf+key: %w", err)
	}

	// Compute Leaf (parsed cert) so downstream consumers don't re-parse,
	// and re-derive ClusterID from its SPIFFE URI. ClusterID is needed
	// by SeedBundleFromAgentDisk and CSR generation; on a restart boot
	// the runtime's pkiState starts with an empty ClusterID, so pulling
	// it from the leaf here is what keeps the listener's trust bundle
	// accessor honest before raft replication kicks in.
	if leaf, err := x509.ParseCertificate(c.Certificate[0]); err == nil {
		c.Leaf = leaf

		if cid, err := pki.ClusterIDFromLeaf(leaf); err == nil {
			a.ClusterID = cid
		}
	}

	a.current.Store(&c)

	return nil
}

// StoreLeaf writes leafPEM, keyPEM, and bundlePEM to disk atomically
// and builds the in-memory tls.Certificate (leaf + bundle + key) used
// by the listener.
func (a *Agent) StoreLeaf(leafPEM, keyPEM, bundlePEM []byte) error {
	if err := os.MkdirAll(filepath.Dir(a.path(leafCertFile)), 0o700); err != nil {
		return fmt.Errorf("mkdir cluster-tls: %w", err)
	}

	for _, f := range []struct {
		name string
		data []byte
		mode os.FileMode
	}{
		{leafKeyFile, keyPEM, 0o600},
		{leafCertFile, leafPEM, 0o644},
		{bundleCertFile, bundlePEM, 0o644},
	} {
		if err := atomicWrite(a.path(f.name), f.data, f.mode); err != nil {
			return err
		}
	}

	chainPEM := append(append([]byte{}, leafPEM...), bundlePEM...)

	c, err := tls.X509KeyPair(chainPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("pair leaf+key after store: %w", err)
	}

	if leaf, err := x509.ParseCertificate(c.Certificate[0]); err == nil {
		c.Leaf = leaf
	}

	a.current.Store(&c)

	return nil
}

// IssueResponse is the wire format returned by /cluster/pki/issue.
type IssueResponse struct {
	LeafPEM   string `json:"leafPEM"`
	BundlePEM string `json:"bundlePEM"`
}

// IssueRequest is the wire format posted to /cluster/pki/issue.
type IssueRequest struct {
	CSRPEM    string `json:"csrPEM"`
	Token     string `json:"token,omitempty"`
	NodeID    int    `json:"nodeID"`
	ClusterID string `json:"clusterID,omitempty"`
}

// JoinFlow dials serverAddr on the bootstrap ALPN, pins the server cert
// to the CA fingerprint embedded in the join token, submits a CSR +
// token, and writes the returned leaf and trust bundle to disk. The
// fingerprint is extracted from the token's claims without HMAC
// verification; tampering is caught later when the server validates
// the HMAC, and a bad fingerprint already aborts the TLS handshake.
// Generates a fresh leaf key.
func (a *Agent) JoinFlow(ctx context.Context, serverAddr, token string) error {
	claims, err := pki.ExtractClaimsUnverified(token)
	if err != nil {
		return fmt.Errorf("parse join token: %w", err)
	}

	if claims.CAFingerprint == "" {
		return fmt.Errorf("join token has no CA fingerprint")
	}

	if claims.ClusterID == "" {
		return fmt.Errorf("join token has no cluster id")
	}

	if claims.NodeID != a.NodeID {
		// Token was minted for a different node-id. Fail early rather
		// than wait for the server to reject on node-id mismatch.
		return fmt.Errorf("join token is for node-id %d, but this node is %d", claims.NodeID, a.NodeID)
	}

	// Seed the agent's clusterID from the token so postIssue can build
	// a CSR with the correct SPIFFE URI on first boot. The server will
	// still reject a mismatched clusterID when it verifies the token's
	// HMAC, so a forged token here doesn't bypass the check.
	if a.ClusterID == "" {
		a.ClusterID = claims.ClusterID
	}

	client, err := bootstrapHTTPClient(claims.CAFingerprint)
	if err != nil {
		return err
	}

	defer client.CloseIdleConnections()

	return a.postIssue(ctx, client, "https://"+serverAddr+"/cluster/pki/issue", token)
}

// RenewFlow posts a fresh CSR to the leader's /cluster/pki/renew
// endpoint over the existing mTLS cluster listener and installs the
// returned leaf.
func (a *Agent) RenewFlow(ctx context.Context, ln *listener.Listener, leaderAddr string, leaderID int) error {
	client := &http.Client{
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return ln.Dial(ctx, addr, leaderID, listener.ALPNHTTP, 10*time.Second)
			},
		},
		Timeout: 30 * time.Second,
	}

	defer client.CloseIdleConnections()

	return a.postIssue(ctx, client, "https://"+leaderAddr+"/cluster/pki/renew", "")
}

// postIssue generates a CSR, POSTs it to url, and stores the returned
// leaf + bundle. Shared between JoinFlow (bootstrap ALPN, token auth)
// and RenewFlow (mTLS, cert auth).
func (a *Agent) postIssue(ctx context.Context, client *http.Client, url, token string) error {
	keyPEM, csrPEM, err := pki.GenerateKeyAndCSR(a.NodeID, a.ClusterID)
	if err != nil {
		return fmt.Errorf("generate csr: %w", err)
	}

	bodyBytes, err := json.Marshal(IssueRequest{
		CSRPEM:    string(csrPEM),
		Token:     token,
		NodeID:    a.NodeID,
		ClusterID: a.ClusterID,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req) // #nosec G107 -- url built from operator-configured peer or Raft-tracked leader address
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s returned %d: %s", url, resp.StatusCode, string(msg))
	}

	var out IssueResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if out.LeafPEM == "" || out.BundlePEM == "" {
		return fmt.Errorf("%s response missing leaf or bundle", url)
	}

	return a.StoreLeaf([]byte(out.LeafPEM), keyPEM, []byte(out.BundlePEM))
}

// RenewSchedule returns the soft and hard renewal times for the current
// leaf, per the §3.4 jitter policy. Zero times are returned if no leaf
// is loaded.
func (a *Agent) RenewSchedule(now time.Time) (soft, hard time.Time) {
	c := a.current.Load()
	if c == nil || c.Leaf == nil {
		return time.Time{}, time.Time{}
	}

	total := c.Leaf.NotAfter.Sub(c.Leaf.NotBefore)

	return c.Leaf.NotBefore.Add(time.Duration(float64(total) * 0.6)),
		c.Leaf.NotBefore.Add(time.Duration(float64(total) * 0.9))
}

// PickRenewAt returns a uniform-random time in [soft, hard) given now,
// or now itself if we're already past hard. If no leaf is loaded,
// returns now+1h as a conservative retry.
func (a *Agent) PickRenewAt(now time.Time) time.Time {
	soft, hard := a.RenewSchedule(now)
	if soft.IsZero() {
		return now.Add(time.Hour)
	}

	if now.After(hard) {
		return now
	}

	lower := soft
	if now.After(lower) {
		lower = now
	}

	span := hard.Sub(lower)
	if span <= 0 {
		return lower
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(span)))
	if err != nil {
		return lower
	}

	return lower.Add(time.Duration(n.Int64()))
}

func (a *Agent) path(name string) string {
	return filepath.Join(a.DataDir, "cluster-tls", name)
}

// BundlePath returns the filesystem path of bundle.crt. The runtime
// reads it at startup to seed the listener's trust cache before raft
// replication makes the CA tables locally queryable.
func (a *Agent) BundlePath() string {
	return a.path(bundleCertFile)
}

// atomicWrite writes data to path+".tmp" then renames, so readers never
// see a half-written file.
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	return nil
}

// bootstrapHTTPClient returns an HTTP client that dials the bootstrap
// ALPN without a client cert and pins the server cert fingerprint.
func bootstrapHTTPClient(expectedFingerprint string) (*http.Client, error) {
	expected := strings.TrimPrefix(expectedFingerprint, "sha256:")

	raw, err := hex.DecodeString(expected)
	if err != nil {
		return nil, fmt.Errorf("decode fingerprint %q: %w", expectedFingerprint, err)
	}

	if len(raw) != sha256.Size {
		return nil, fmt.Errorf("fingerprint length %d, want %d", len(raw), sha256.Size)
	}

	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		NextProtos:         []string{listener.ALPNPKIBootstrap},
		InsecureSkipVerify: true, // #nosec G402 -- VerifyConnection pins the fingerprint
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return fmt.Errorf("bootstrap: server presented no cert")
			}

			for _, c := range cs.PeerCertificates {
				if bytes.Equal(certFingerprint(c), raw) {
					return nil
				}
			}

			seen := make([]string, 0, len(cs.PeerCertificates))
			for _, c := range cs.PeerCertificates {
				seen = append(seen, fmt.Sprintf("%s=sha256:%s", c.Subject.CommonName, hex.EncodeToString(certFingerprint(c))))
			}

			return fmt.Errorf("bootstrap: server chain (%d certs: %v) does not contain pinned %s", len(cs.PeerCertificates), seen, expectedFingerprint)
		},
	}

	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig:       tlsCfg,
			ForceAttemptHTTP2:     false,
			ResponseHeaderTimeout: 10 * time.Second,
		},
	}, nil
}

func certFingerprint(c *x509.Certificate) []byte {
	sum := sha256.Sum256(c.Raw)
	return sum[:]
}
