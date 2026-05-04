// Copyright 2026 Ella Networks

// Package pkiissuer is the leader-side service that registers a
// joining node's self-signed cluster cert. Unlike the legacy CA-based
// issuer, this service does not sign anything: it validates the join
// token, parses the offered cert, confirms the SPIFFE URI's nodeID
// matches the token's claims, and replicates the pin into
// cluster_node_certs via Raft. The runtime then refreshes its in-
// memory pin map so subsequent handshakes from the new node are
// accepted.
package pkiissuer

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/pki"
)

// Store is the narrow DB surface the issuer needs.
type Store interface {
	GetOperator(ctx context.Context) (*db.Operator, error)

	GetClusterJoinHMACKey(ctx context.Context) ([]byte, error)
	InitClusterJoinHMACKey(ctx context.Context, key []byte) error

	UpsertClusterNodeCert(ctx context.Context, r *db.ClusterNodeCert) error
	ListClusterNodeCerts(ctx context.Context) ([]db.ClusterNodeCert, error)

	MintJoinTokenRecord(ctx context.Context, r *db.ClusterJoinToken) error
	GetJoinToken(ctx context.Context, id string) (*db.ClusterJoinToken, error)
	ConsumeJoinToken(ctx context.Context, id string, nodeID int) error

	IsLeader() bool
}

// Service runs on every voter. Mutating operations (Bootstrap,
// MintJoinToken, RegisterCert) require IsLeader; read paths
// (ListPins) work on followers too.
type Service struct {
	store Store
}

func New(store Store) *Service {
	return &Service{store: store}
}

// Bootstrap is called from the leader-init path. It seeds the
// HMAC-key singleton (idempotent) so MintJoinToken can sign tokens.
// Trivially fast; safe to retry.
func (s *Service) Bootstrap(ctx context.Context) error {
	if !s.store.IsLeader() {
		return fmt.Errorf("not leader")
	}

	if _, err := s.store.GetClusterJoinHMACKey(ctx); err == nil {
		return nil
	} else if !errors.Is(err, db.ErrNotFound) {
		return fmt.Errorf("get hmac key: %w", err)
	}

	key, err := newHMACKey()
	if err != nil {
		return fmt.Errorf("generate hmac key: %w", err)
	}

	if err := s.store.InitClusterJoinHMACKey(ctx, key); err != nil {
		return fmt.Errorf("init hmac key: %w", err)
	}

	return nil
}

// Ready reports whether the service can mint or verify join tokens.
// A follower returns true once the HMAC key has replicated; a leader
// returns true once Bootstrap has run.
func (s *Service) Ready(ctx context.Context) bool {
	if _, err := s.store.GetClusterJoinHMACKey(ctx); err != nil {
		return false
	}

	return true
}

// MintJoinToken emits a single-use HMAC token bound to nodeID with
// the given TTL, embedding the leader's own pinned cert fingerprint
// so the joining node can pin the bootstrap TLS handshake.
//
// leaderCertPin is the operator-supplied fingerprint of the leader's
// self-signed cert (taken from cluster_node_certs WHERE nodeID =
// leader's nodeID). If empty, MintJoinToken looks it up from the
// replicated table.
func (s *Service) MintJoinToken(ctx context.Context, nodeID int, ttl time.Duration, leaderNodeID int) (string, error) {
	if ttl < pki.DefaultJoinTokenMinTTL || ttl > pki.DefaultJoinTokenMaxTTL {
		return "", fmt.Errorf("join-token ttl %s outside [%s, %s]", ttl, pki.DefaultJoinTokenMinTTL, pki.DefaultJoinTokenMaxTTL)
	}

	if !s.store.IsLeader() {
		return "", fmt.Errorf("not leader")
	}

	hmacKey, err := s.store.GetClusterJoinHMACKey(ctx)
	if err != nil {
		return "", fmt.Errorf("get hmac key: %w", err)
	}

	op, err := s.store.GetOperator(ctx)
	if err != nil {
		return "", fmt.Errorf("get operator: %w", err)
	}

	if op.ClusterID == "" {
		return "", fmt.Errorf("cluster id not yet populated")
	}

	leaderPin, err := s.leaderPin(ctx, leaderNodeID)
	if err != nil {
		return "", err
	}

	tokenID, err := pki.NewTokenID()
	if err != nil {
		return "", err
	}

	now := time.Now()

	claims := pki.JoinClaims{
		TokenID:       tokenID,
		NodeID:        nodeID,
		IssuedAt:      now.Unix(),
		ExpiresAt:     now.Add(ttl).Unix(),
		LeaderCertPin: leaderPin,
		ClusterID:     op.ClusterID,
	}

	tokenStr, err := pki.MintJoinToken(hmacKey, claims)
	if err != nil {
		return "", err
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("encode claims: %w", err)
	}

	if err := s.store.MintJoinTokenRecord(ctx, &db.ClusterJoinToken{
		ID:         tokenID,
		NodeID:     nodeID,
		ClaimsJSON: string(claimsJSON),
		ExpiresAt:  claims.ExpiresAt,
	}); err != nil {
		return "", fmt.Errorf("record join token: %w", err)
	}

	return tokenStr, nil
}

func (s *Service) leaderPin(ctx context.Context, leaderNodeID int) (string, error) {
	rows, err := s.store.ListClusterNodeCerts(ctx)
	if err != nil {
		return "", fmt.Errorf("list pins: %w", err)
	}

	if leaderNodeID == 0 {
		// Standalone / single-node test path: there is at most one
		// registered pin (the local node), and it is by definition
		// the leader. Pick it.
		if len(rows) == 1 {
			return rows[0].Fingerprint, nil
		}

		return "", fmt.Errorf("leaderNodeID is zero and registry has %d pins", len(rows))
	}

	for _, r := range rows {
		if r.NodeID == leaderNodeID {
			return r.Fingerprint, nil
		}
	}

	return "", fmt.Errorf("leader node %d has no registered pin", leaderNodeID)
}

// VerifyAndConsumeJoinToken authenticates tokenStr, enforces expiry
// and single-use semantics, and records consumption.
func (s *Service) VerifyAndConsumeJoinToken(ctx context.Context, tokenStr string) (*pki.JoinClaims, error) {
	hmacKey, err := s.store.GetClusterJoinHMACKey(ctx)
	if err != nil {
		return nil, err
	}

	claims, err := pki.VerifyJoinToken(hmacKey, time.Now(), tokenStr)
	if err != nil {
		return nil, err
	}

	row, err := s.store.GetJoinToken(ctx, claims.TokenID)
	if err != nil {
		return nil, fmt.Errorf("lookup join token: %w", err)
	}

	if row.ConsumedAt != 0 {
		return nil, fmt.Errorf("token already consumed")
	}

	if err := s.store.ConsumeJoinToken(ctx, claims.TokenID, claims.NodeID); err != nil {
		if errors.Is(err, db.ErrJoinTokenAlreadyConsumed) {
			return nil, fmt.Errorf("token already consumed")
		}

		return nil, fmt.Errorf("consume join token: %w", err)
	}

	return claims, nil
}

// RegisterCert pins certPEM under nodeID. The cert must be a
// well-formed cluster cert (SPIFFE URI matches op.ClusterID and
// declares nodeID). Driven by the /cluster/pki/register handler
// after token verification or after a peer-mTLS rotation request.
func (s *Service) RegisterCert(ctx context.Context, nodeID int, certPEM []byte) (string, error) {
	if !s.store.IsLeader() {
		return "", fmt.Errorf("not leader")
	}

	op, err := s.store.GetOperator(ctx)
	if err != nil {
		return "", fmt.Errorf("get operator: %w", err)
	}

	cert, err := pki.ParseCertPEM(certPEM)
	if err != nil {
		return "", fmt.Errorf("parse cert: %w", err)
	}

	clusterID, certNodeID, err := pki.IdentityFromCert(cert)
	if err != nil {
		return "", fmt.Errorf("invalid cluster cert: %w", err)
	}

	if clusterID != op.ClusterID {
		return "", fmt.Errorf("cert clusterID %q != operator clusterID %q", clusterID, op.ClusterID)
	}

	if certNodeID != nodeID {
		return "", fmt.Errorf("cert URI nodeID %d != requested nodeID %d", certNodeID, nodeID)
	}

	// Self-signed sanity check: issuer must equal subject. Catches a
	// cert minted by an unrelated CA being slipped past register.
	if !bytesEqual(cert.RawIssuer, cert.RawSubject) {
		return "", fmt.Errorf("cert is not self-signed")
	}

	if err := cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature); err != nil {
		return "", fmt.Errorf("self-signature verify: %w", err)
	}

	fp := pki.Fingerprint(cert)

	row := &db.ClusterNodeCert{
		NodeID:      nodeID,
		Fingerprint: fp,
		CertPEM:     string(pki.EncodeCertPEM(cert)),
		AddedAt:     time.Now().Unix(),
	}

	if err := s.store.UpsertClusterNodeCert(ctx, row); err != nil {
		return "", fmt.Errorf("upsert pin: %w", err)
	}

	return fp, nil
}

// CurrentPins returns the replicated pin set, suitable for caching
// in the listener's PinFunc.
func (s *Service) CurrentPins(ctx context.Context) (map[string]int, error) {
	rows, err := s.store.ListClusterNodeCerts(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[string]int, len(rows))
	for _, r := range rows {
		out[r.Fingerprint] = r.NodeID
	}

	return out, nil
}

// IsLeader reflects the underlying store. Used by handlers that need
// a leader-only short-circuit.
func (s *Service) IsLeader() bool { return s.store.IsLeader() }

func newHMACKey() ([]byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("rand: %w", err)
	}

	return b, nil
}

// bytesEqual avoids importing bytes just for one call.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
