// Copyright 2026 Ella Networks

package runtime

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// keyTransferInterval is how often the voter-side worker retries if no
// peer served the keys. Intentionally short: during a leader-handover
// window we want a promoted voter to already hold the keys.
const keyTransferInterval = 15 * time.Second

// runKeyTransferWorker runs on every voter. If this node lacks the PKI
// signing material on disk, it periodically dials other voter members
// over mTLS and pulls the keys via GET /cluster/pki/keys. On success,
// validates that the received public keys match the replicated active
// root and intermediate certs, then persists them to disk.
//
// Exits immediately once keys are present (the bootstrap voter never
// enters the retry loop because it wrote the keys during Bootstrap).
// The worker is idempotent: re-running after keys are imported is a
// no-op.
func runKeyTransferWorker(ctx context.Context, pki *pkiState, ln *listener.Listener, dbInstance *db.Database, selfNodeID int) {
	// Small initial delay so join-flow / discovery / PKI-issuer install
	// complete before we start asking peers for material.
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second):
	}

	t := time.NewTicker(keyTransferInterval)
	defer t.Stop()

	for {
		if pki.issuer != nil && pki.issuer.HaveKeysOnDisk() {
			return
		}

		if err := tryFetchKeysFromPeer(ctx, pki, ln, dbInstance, selfNodeID); err != nil {
			logger.EllaLog.Debug("key transfer: no peer served yet", zap.Error(err))
		} else if pki.issuer != nil && pki.issuer.HaveKeysOnDisk() {
			logger.EllaLog.Info("key transfer: signing material imported from peer")
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func tryFetchKeysFromPeer(ctx context.Context, pki *pkiState, ln *listener.Listener, dbInstance *db.Database, selfNodeID int) error {
	if pki.issuer == nil {
		return fmt.Errorf("issuer not yet installed")
	}

	members, err := dbInstance.ListClusterMembers(ctx)
	if err != nil {
		return fmt.Errorf("list cluster members: %w", err)
	}

	var lastErr error

	for _, m := range members {
		if m.NodeID == selfNodeID || m.Suffrage != "voter" {
			continue
		}

		if err := fetchKeysFrom(ctx, pki, ln, m); err != nil {
			lastErr = fmt.Errorf("node %d (%s): %w", m.NodeID, m.RaftAddress, err)
			continue
		}

		return nil
	}

	if lastErr == nil {
		return fmt.Errorf("no voter peers available")
	}

	return lastErr
}

func fetchKeysFrom(ctx context.Context, pki *pkiState, ln *listener.Listener, m db.ClusterMember) error {
	client := &http.Client{
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return ln.Dial(ctx, addr, m.NodeID, listener.ALPNHTTP, 10*time.Second)
			},
		},
		Timeout: 20 * time.Second,
	}

	defer client.CloseIdleConnections()

	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(dialCtx, http.MethodGet,
		"https://"+m.RaftAddress+"/cluster/pki/keys", nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req) // #nosec G107 -- URL built from replicated member address
	if err != nil {
		return fmt.Errorf("dial/get: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var out server.KeyTransferResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&out); err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	if out.RootKeyPEM == "" || out.IntermediateKeyPEM == "" {
		return fmt.Errorf("empty key PEM(s) in response")
	}

	// Validate received keys match the replicated active root and
	// intermediate certs before persisting, so a compromised or
	// malfunctioning peer cannot hand us a pair of keys that don't
	// match the cluster's trust state.
	if err := validateKeyMatchesBundle(ctx, pki, out); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if err := pki.issuer.ImportKeys([]byte(out.RootKeyPEM), []byte(out.IntermediateKeyPEM)); err != nil {
		return fmt.Errorf("import: %w", err)
	}

	return nil
}

func validateKeyMatchesBundle(ctx context.Context, pki *pkiState, out server.KeyTransferResponse) error {
	bundle, err := pki.issuer.CurrentBundle(ctx)
	if err != nil {
		return fmt.Errorf("current bundle: %w", err)
	}

	rootSigner, err := parseKeyPEM([]byte(out.RootKeyPEM))
	if err != nil {
		return fmt.Errorf("root key: %w", err)
	}

	if err := publicKeyInBundle(rootSigner, bundle.Roots); err != nil {
		return fmt.Errorf("root key does not match any trusted root cert: %w", err)
	}

	intSigner, err := parseKeyPEM([]byte(out.IntermediateKeyPEM))
	if err != nil {
		return fmt.Errorf("intermediate key: %w", err)
	}

	if err := publicKeyInBundle(intSigner, bundle.Intermediates); err != nil {
		return fmt.Errorf("intermediate key does not match any trusted intermediate cert: %w", err)
	}

	return nil
}

func parseKeyPEM(raw []byte) (any, error) {
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("not a PRIVATE KEY PEM")
	}

	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse pkcs8: %w", err)
	}

	return k, nil
}

func publicKeyInBundle(signer any, certs []*x509.Certificate) error {
	var pub any

	switch k := signer.(type) {
	case *ecdsa.PrivateKey:
		pub = &k.PublicKey
	case *rsa.PrivateKey:
		pub = &k.PublicKey
	default:
		return fmt.Errorf("unsupported key type %T", signer)
	}

	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return err
	}

	for _, c := range certs {
		certPubDER, err := x509.MarshalPKIXPublicKey(c.PublicKey)
		if err != nil {
			continue
		}

		if string(certPubDER) == string(pubDER) {
			return nil
		}
	}

	return fmt.Errorf("no matching cert in bundle (%d certs)", len(certs))
}

// keyTransferEnabled reports whether this node should run the key
// transfer worker. Non-voters never become leader, so they do not need
// the signing material. We call the helper with the config value rather
// than peeking at the cluster state to avoid a race during initial
// boot, when the cluster_members row has not yet been written.
func keyTransferEnabled(initialSuffrage string) bool {
	return initialSuffrage == "" || initialSuffrage == "voter"
}
