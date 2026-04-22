// Copyright 2026 Ella Networks

// Package pkiagent runs on every cluster node. It owns the local leaf
// key, fetches a leaf at first boot (via the join flow on a joining
// node, or via the issuer service on the first node), persists the leaf
// and trust bundle to disk, and renews on a jittered schedule.
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

// Filenames under <dataDir>/cluster-tls/. Trust bundle is no longer
// persisted to disk — it's derived at startup from the replicated
// cluster_pki_roots and cluster_pki_intermediates tables.
const (
	leafCertFile = "leaf.crt"
	leafKeyFile  = "leaf.key"
)

// Agent manages the local node's cluster leaf.
type Agent struct {
	NodeID    int
	ClusterID string
	DataDir   string

	current atomic.Pointer[tls.Certificate]

	// onBundle, if non-nil, is invoked with the trust bundle PEM
	// returned by JoinFlow / RenewFlow before StoreLeaf writes the leaf.
	// Callers use this to seed the listener's trust cache on first
	// boot, before raft replication has made the CA tables locally
	// readable. Refreshes from the DB replace this seed once raft
	// catches up.
	onBundle func(bundlePEM []byte)
}

// NewAgent returns an unloaded agent. Callers should call Load or
// perform an initial JoinFlow / SelfIssue before relying on Leaf().
func NewAgent(nodeID int, clusterID, dataDir string) *Agent {
	return &Agent{
		NodeID:    nodeID,
		ClusterID: clusterID,
		DataDir:   dataDir,
	}
}

// SetOnBundle registers a callback invoked with the trust bundle PEM on
// every successful JoinFlow / RenewFlow. Runtime wires this into the
// trust cache so the listener has peers' root of trust before raft
// replication has made the CA tables locally readable.
func (a *Agent) SetOnBundle(fn func(bundlePEM []byte)) {
	a.onBundle = fn
}

// Leaf returns the current leaf certificate, or nil if none has been
// loaded. Safe for use as listener.Config.Leaf.
func (a *Agent) Leaf() *tls.Certificate {
	return a.current.Load()
}

// HaveLeafOnDisk reports whether leaf.crt and leaf.key exist in the
// cluster-tls directory.
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

// Load reads leaf.crt and leaf.key from disk and makes them the current
// leaf. The trust bundle is no longer persisted to disk; callers who
// need it for trust validation derive it from the replicated
// cluster_pki_{roots,intermediates} tables. The returned tls.Certificate
// therefore carries only the leaf — peers receive their own chain via
// the same replicated state during handshake, and the leaf's issuer is
// implicit.
//
// bundlePEM is the PEM-encoded roots+intermediates for this cluster; it
// is appended to the leaf so the resulting tls.Certificate carries the
// full chain on the wire, matching what peers expect during mTLS
// verification. Pass nil when a chain is not required (tests only).
func (a *Agent) Load(bundlePEM []byte) error {
	certPEM, err := os.ReadFile(a.path(leafCertFile)) // #nosec: G304 -- under dataDir
	if err != nil {
		return fmt.Errorf("read leaf.crt: %w", err)
	}

	keyPEM, err := os.ReadFile(a.path(leafKeyFile)) // #nosec: G304
	if err != nil {
		return fmt.Errorf("read leaf.key: %w", err)
	}

	chainPEM := certPEM
	if len(bundlePEM) > 0 {
		chainPEM = append(append([]byte{}, certPEM...), bundlePEM...)
	}

	c, err := tls.X509KeyPair(chainPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("load leaf+key: %w", err)
	}

	// Compute Leaf (parsed cert) so downstream consumers don't re-parse.
	if leaf, err := x509.ParseCertificate(c.Certificate[0]); err == nil {
		c.Leaf = leaf
	}

	a.current.Store(&c)

	return nil
}

// StoreLeaf persists leafPEM and keyPEM to disk atomically and sets the
// in-memory tls.Certificate (leaf+bundle+key) for the listener. The
// bundle PEM is held in memory only — it's not persisted, since roots
// and intermediates are already replicated via the DB and can be
// reassembled from there on every restart.
//
// Before returning, StoreLeaf invokes the onBundle callback (if
// registered) with the bundle PEM so runtime can seed the listener's
// trust cache during a join, before raft replication has caught up on
// this node.
//
// The in-memory tls.Certificate carries the full chain so the server
// can present it in one TLS handshake.
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

	if a.onBundle != nil {
		a.onBundle(bundlePEM)
	}

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
