// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package pkiissuer is the leader-side service for join-token
// minting and cluster-certificate registration. RegisterCert
// validates a node-supplied self-signed certificate (SPIFFE URI
// shape, clusterID, requested nodeID, self-signature) and
// replicates its SHA-256 pin into cluster_node_certs via Raft;
// MintJoinToken issues HMAC-signed single-use tokens that admit a
// joining node to call RegisterCert.
package pkiissuer

import (
	"bytes"
	"context"
	"crypto/x509"
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
	RedeemJoinToken(ctx context.Context, tokenID string, nodeID int, fingerprint, certPEM string) ([]db.ClusterNodeCert, error)

	IsLeader() bool
}

// Service runs on every voter. Bootstrap and MintJoinToken require
// IsLeader; RedeemJoinToken and RegisterCert forward to the leader.
type Service struct {
	store Store
}

func New(store Store) *Service {
	return &Service{store: store}
}

// Bootstrap seeds the HMAC-key singleton on the leader-init path.
// Idempotent; re-runs on every leader transition are no-ops once
// the row exists.
func (s *Service) Bootstrap(ctx context.Context) error {
	if !s.store.IsLeader() {
		return fmt.Errorf("not leader")
	}

	if _, err := s.store.GetClusterJoinHMACKey(ctx); err == nil {
		return nil
	} else if !errors.Is(err, db.ErrNotFound) {
		return fmt.Errorf("get hmac key: %w", err)
	}

	key, err := pki.NewHMACKey()
	if err != nil {
		return fmt.Errorf("generate hmac key: %w", err)
	}

	if err := s.store.InitClusterJoinHMACKey(ctx, key); err != nil {
		return fmt.Errorf("init hmac key: %w", err)
	}

	return nil
}

// Ready reports whether the service can mint or verify join
// tokens. Becomes true on the leader once Bootstrap commits, and
// on followers once the HMAC key replicates.
func (s *Service) Ready(ctx context.Context) bool {
	if _, err := s.store.GetClusterJoinHMACKey(ctx); err != nil {
		return false
	}

	return true
}

// MintJoinToken emits a single-use HMAC token bound to nodeID with
// the given TTL, embedding the cluster_node_certs pin owned by
// leaderNodeID so the joining node pins the bootstrap TLS
// handshake against the leader's certificate.
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

	leaderPin, allPins, err := s.pinsForToken(ctx, leaderNodeID)
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
		ClusterPins:   allPins,
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

func (s *Service) pinsForToken(ctx context.Context, leaderNodeID int) (string, []string, error) {
	rows, err := s.store.ListClusterNodeCerts(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("list pins: %w", err)
	}

	all := make([]string, 0, len(rows))
	for _, r := range rows {
		all = append(all, r.Fingerprint)
	}

	if leaderNodeID == 0 {
		// Standalone or single-node test path: with exactly one
		// registered pin, that pin belongs to the leader.
		if len(rows) == 1 {
			return rows[0].Fingerprint, all, nil
		}

		return "", nil, fmt.Errorf("leaderNodeID is zero and registry has %d pins", len(rows))
	}

	for _, r := range rows {
		if r.NodeID == leaderNodeID {
			return r.Fingerprint, all, nil
		}
	}

	return "", nil, fmt.Errorf("leader node %d has no registered pin", leaderNodeID)
}

func (s *Service) RedeemJoinToken(ctx context.Context, tokenStr string, nodeID int, certPEM []byte) (string, []db.ClusterNodeCert, error) {
	hmacKey, err := s.store.GetClusterJoinHMACKey(ctx)
	if err != nil {
		return "", nil, err
	}

	claims, err := pki.VerifyJoinToken(hmacKey, time.Now(), tokenStr)
	if err != nil {
		return "", nil, err
	}

	if claims.NodeID != nodeID {
		return "", nil, fmt.Errorf("token is for node %d, not %d", claims.NodeID, nodeID)
	}

	cert, fp, err := s.validateNodeCert(ctx, nodeID, certPEM)
	if err != nil {
		return "", nil, err
	}

	pins, err := s.store.RedeemJoinToken(ctx, claims.TokenID, nodeID, fp, string(pki.EncodeCertPEM(cert)))
	if err != nil {
		return "", nil, err
	}

	return fp, pins, nil
}

// RegisterCert validates certPEM (SPIFFE URI matches the cluster's
// clusterID, declares nodeID, self-signed and self-signature
// verifies) and replicates its SHA-256 pin into cluster_node_certs.
// Returns the pin fingerprint and the post-commit snapshot of
// every registered pin so the caller can seed its local pin map.
func (s *Service) RegisterCert(ctx context.Context, nodeID int, certPEM []byte) (string, []db.ClusterNodeCert, error) {
	cert, fp, err := s.validateNodeCert(ctx, nodeID, certPEM)
	if err != nil {
		return "", nil, err
	}

	row := &db.ClusterNodeCert{
		NodeID:      nodeID,
		Fingerprint: fp,
		CertPEM:     string(pki.EncodeCertPEM(cert)),
		AddedAt:     time.Now().Unix(),
	}

	if err := s.store.UpsertClusterNodeCert(ctx, row); err != nil {
		return "", nil, fmt.Errorf("upsert pin: %w", err)
	}

	pins, err := s.store.ListClusterNodeCerts(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("list pins after commit: %w", err)
	}

	return fp, pins, nil
}

func (s *Service) validateNodeCert(ctx context.Context, nodeID int, certPEM []byte) (*x509.Certificate, string, error) {
	op, err := s.store.GetOperator(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("get operator: %w", err)
	}

	cert, err := pki.ParseCertPEM(certPEM)
	if err != nil {
		return nil, "", fmt.Errorf("parse cert: %w", err)
	}

	clusterID, certNodeID, err := pki.IdentityFromCert(cert)
	if err != nil {
		return nil, "", fmt.Errorf("invalid cluster cert: %w", err)
	}

	if clusterID != op.ClusterID {
		return nil, "", fmt.Errorf("cert clusterID %q != operator clusterID %q", clusterID, op.ClusterID)
	}

	if certNodeID != nodeID {
		return nil, "", fmt.Errorf("cert URI nodeID %d != requested nodeID %d", certNodeID, nodeID)
	}

	// Issuer must equal subject; the cluster TLS contract requires
	// every node cert to be self-signed.
	if !bytes.Equal(cert.RawIssuer, cert.RawSubject) {
		return nil, "", fmt.Errorf("cert is not self-signed")
	}

	if err := cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature); err != nil {
		return nil, "", fmt.Errorf("self-signature verify: %w", err)
	}

	return cert, pki.Fingerprint(cert), nil
}
