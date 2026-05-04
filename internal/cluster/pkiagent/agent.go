// Copyright 2026 Ella Networks

// Package pkiagent runs on every cluster node. It owns the local
// self-signed cluster certificate: generates it at first boot,
// persists it to disk, and POSTs the certificate (plus a join
// token, on a fresh node) to the leader's /cluster/pki/register
// endpoint so the leader replicates the pin to every voter.
// Optional rotation re-runs the same generate-and-register flow;
// the pre-rotation pin remains valid until the new one commits, so
// rotation is safe to retry.
package pkiagent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/pki"
)

// Filenames under <dataDir>/cluster-tls/.
const (
	leafCertFile = "leaf.crt"
	leafKeyFile  = "leaf.key"
)

// Agent manages the local node's cluster cert.
type Agent struct {
	NodeID    int
	ClusterID string
	DataDir   string

	current atomic.Pointer[tls.Certificate]
}

// NewAgent returns an unloaded agent. Callers must invoke Load,
// JoinFlow, or GenerateAndPersist before Leaf returns a usable
// certificate.
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
	if _, err := os.Stat(a.path(leafCertFile)); err != nil {
		return false
	}

	if _, err := os.Stat(a.path(leafKeyFile)); err != nil {
		return false
	}

	return true
}

// Load reads the on-disk cert and key into memory. Idempotent.
func (a *Agent) Load() error {
	certPEM, err := os.ReadFile(a.path(leafCertFile)) // #nosec G304 -- under dataDir
	if err != nil {
		return fmt.Errorf("read leaf.crt: %w", err)
	}

	keyPEM, err := os.ReadFile(a.path(leafKeyFile)) // #nosec G304
	if err != nil {
		return fmt.Errorf("read leaf.key: %w", err)
	}

	c, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("load leaf+key: %w", err)
	}

	if leaf, err := x509.ParseCertificate(c.Certificate[0]); err == nil {
		c.Leaf = leaf

		if cid, _, err := pki.IdentityFromCert(leaf); err == nil {
			a.ClusterID = cid
		}
	}

	a.current.Store(&c)

	return nil
}

// GenerateAndPersist creates a fresh self-signed cluster cert under
// the agent's nodeID/clusterID, writes it to disk atomically, and
// installs it as the live cert for the listener. clusterID must be
// non-empty (set by JoinFlow from the token's claims, or from the
// operator's bootstrap path on the first node).
func (a *Agent) GenerateAndPersist() error {
	if a.ClusterID == "" {
		return fmt.Errorf("cluster id not set")
	}

	cert, key, err := pki.GenerateNodeCert(a.NodeID, a.ClusterID, pki.DefaultNodeCertTTL)
	if err != nil {
		return fmt.Errorf("generate cert: %w", err)
	}

	certPEM := pki.EncodeCertPEM(cert)

	keyPEM, err := pki.EncodePrivateKeyPEM(key)
	if err != nil {
		return fmt.Errorf("encode key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(a.path(leafCertFile)), 0o700); err != nil {
		return fmt.Errorf("mkdir cluster-tls: %w", err)
	}

	if err := atomicWrite(a.path(leafKeyFile), keyPEM, 0o600); err != nil {
		return err
	}

	if err := atomicWrite(a.path(leafCertFile), certPEM, 0o644); err != nil {
		return err
	}

	c, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("pair after store: %w", err)
	}

	c.Leaf = cert

	a.current.Store(&c)

	return nil
}

// CurrentCertPEM returns the PEM encoding of the live cert, suitable
// for posting to the leader's /cluster/pki/register endpoint. Returns
// nil if no cert has been generated yet.
func (a *Agent) CurrentCertPEM() []byte {
	leaf := a.current.Load()
	if leaf == nil || leaf.Leaf == nil {
		return nil
	}

	return pki.EncodeCertPEM(leaf.Leaf)
}

// RegisterRequest is the wire format posted to /cluster/pki/register.
type RegisterRequest struct {
	CertPEM   string `json:"certPEM"`
	Token     string `json:"token,omitempty"`
	NodeID    int    `json:"nodeID"`
	ClusterID string `json:"clusterID"`
}

// RegisterResponse is returned on a successful register.
type RegisterResponse struct {
	Fingerprint string `json:"fingerprint"`
}

// JoinFlow runs on a fresh node. It reads the token's claims (without
// HMAC verification) to obtain the leader's cert pin and the cluster
// id, generates and persists a self-signed cert, dials the leader's
// bootstrap ALPN with the leader-cert pin enforced, and POSTs
// (cert, token) to /cluster/pki/register. After this returns
// successfully the leader has replicated the pin to every voter and
// the agent's cert is usable for cluster mTLS on subsequent ALPNs.
func (a *Agent) JoinFlow(ctx context.Context, serverAddr, token string) error {
	claims, err := pki.ExtractClaimsUnverified(token)
	if err != nil {
		return fmt.Errorf("parse join token: %w", err)
	}

	if claims.LeaderCertPin == "" {
		return fmt.Errorf("join token has no leader cert pin")
	}

	if claims.ClusterID == "" {
		return fmt.Errorf("join token has no cluster id")
	}

	if claims.NodeID != a.NodeID {
		return fmt.Errorf("join token is for node-id %d, but this node is %d", claims.NodeID, a.NodeID)
	}

	if a.ClusterID == "" {
		a.ClusterID = claims.ClusterID
	}

	if err := a.GenerateAndPersist(); err != nil {
		return fmt.Errorf("self-sign: %w", err)
	}

	client, err := bootstrapHTTPClient(claims.LeaderCertPin)
	if err != nil {
		return err
	}

	defer client.CloseIdleConnections()

	return a.postRegister(ctx, client, "https://"+serverAddr+"/cluster/pki/register", token)
}

// Rotate generates a fresh self-signed cert and re-registers it with
// the leader over the existing mTLS cluster listener. Best-effort: the
// previous pin remains valid until the new one is committed, so a
// failed rotation is recoverable.
func (a *Agent) Rotate(ctx context.Context, ln *listener.Listener, leaderAddr string, leaderID int) error {
	if err := a.GenerateAndPersist(); err != nil {
		return fmt.Errorf("self-sign: %w", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialTLSContext: dialFuncForListener(ln, leaderID),
		},
		Timeout: 30 * time.Second,
	}

	defer client.CloseIdleConnections()

	return a.postRegister(ctx, client, "https://"+leaderAddr+"/cluster/pki/register", "")
}

func (a *Agent) postRegister(ctx context.Context, client *http.Client, url, token string) error {
	certPEM := a.CurrentCertPEM()
	if len(certPEM) == 0 {
		return fmt.Errorf("no cert to register")
	}

	body, err := json.Marshal(RegisterRequest{
		CertPEM:   string(certPEM),
		Token:     token,
		NodeID:    a.NodeID,
		ClusterID: a.ClusterID,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req) // #nosec G107 -- url built from operator-configured peer or Raft-tracked leader
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s returned %d: %s", url, resp.StatusCode, string(msg))
	}

	var out RegisterResponse

	_ = json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&out)

	return nil
}

func (a *Agent) path(name string) string {
	return filepath.Join(a.DataDir, "cluster-tls", name)
}

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
// ALPN without a client cert and pins the server cert to
// expectedFingerprint.
func bootstrapHTTPClient(expectedFingerprint string) (*http.Client, error) {
	raw, err := pki.ParseFingerprint(expectedFingerprint)
	if err != nil {
		return nil, err
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
				sum := sha256.Sum256(c.Raw)
				if subtle.ConstantTimeCompare(sum[:], raw) == 1 {
					return nil
				}
			}

			return fmt.Errorf("bootstrap: server cert chain does not contain pinned %s", expectedFingerprint)
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

// dialFuncForListener wraps the listener's mTLS dialer so an
// http.Client can use it for cluster-internal HTTP.
func dialFuncForListener(ln *listener.Listener, peerID int) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, _, addr string) (net.Conn, error) {
		return ln.Dial(ctx, addr, peerID, listener.ALPNHTTP, 10*time.Second)
	}
}
