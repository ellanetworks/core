// Copyright 2026 Ella Networks

// Package pkiissuer is the cluster's PKI issuer service. It runs on the
// Raft leader: generates root+intermediate on first boot, signs leaves
// requested by joining or renewing nodes, mints and verifies join
// tokens, and drives CA rotation.
//
// All PKI material — including the root and intermediate signing keys —
// lives in the replicated DB (cluster_pki_roots.keyPEM and
// cluster_pki_intermediates.keyPEM on active rows). Every voter
// receives the signing keys through ordinary raft replication; there
// is no separate key-transfer protocol.
package pkiissuer

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

// Store is the narrow DB surface the issuer needs. *db.Database satisfies
// it; keeping it an interface lets tests stub the storage.
type Store interface {
	GetOperator(ctx context.Context) (*db.Operator, error)
	GetPKIState(ctx context.Context) (*db.ClusterPKIState, error)
	BootstrapPKI(ctx context.Context, payload *db.PKIBootstrap) error
	AllocatePKISerial(ctx context.Context) (int64, error)

	ListPKIRoots(ctx context.Context) ([]db.ClusterPKIRoot, error)
	ListPKIIntermediates(ctx context.Context) ([]db.ClusterPKIIntermediate, error)

	RecordIssuedCert(ctx context.Context, r *db.ClusterIssuedCert) error
	ListRevokedCerts(ctx context.Context) ([]db.ClusterRevokedCert, error)

	MintJoinTokenRecord(ctx context.Context, r *db.ClusterJoinToken) error
	GetJoinToken(ctx context.Context, id string) (*db.ClusterJoinToken, error)
	ConsumeJoinToken(ctx context.Context, id string, nodeID int) error

	IsLeader() bool
}

// Service is the leader-side PKI issuer.
type Service struct {
	store Store

	mu               sync.RWMutex
	rootKey          crypto.Signer
	intermediateKey  crypto.Signer
	intermediateCert *x509.Certificate
	clusterID        string
}

// New returns an unloaded Service. Call Bootstrap on first-leader boot
// (from PostInitClusterSetup) and LoadKeys on every OnBecameLeader.
func New(store Store) *Service {
	return &Service{store: store}
}

// Bootstrap generates root + intermediate + HMAC key on the first leader
// to run against a fresh cluster. Idempotent: if an active root and
// intermediate already exist, returns nil without touching state.
//
// The three DB writes (pki_state, root, intermediate) land as one raft
// changeset via BootstrapPKI — either all three persist or none do, so
// there is no partial-state window and no self-healing retry branch.
func (s *Service) Bootstrap(ctx context.Context) error {
	op, err := s.store.GetOperator(ctx)
	if err != nil {
		return fmt.Errorf("get operator: %w", err)
	}

	if op.ClusterID == "" {
		return fmt.Errorf("cluster id not yet populated; run PostInitClusterSetup first")
	}

	roots, err := s.store.ListPKIRoots(ctx)
	if err != nil {
		return fmt.Errorf("list roots: %w", err)
	}

	ints, err := s.store.ListPKIIntermediates(ctx)
	if err != nil {
		return fmt.Errorf("list intermediates: %w", err)
	}

	if hasActive(statusesFromRoots(roots)) && hasActive(statusesFromIntermediates(ints)) {
		return nil
	}

	hmacKey, err := pki.NewHMACKey()
	if err != nil {
		return fmt.Errorf("hmac key: %w", err)
	}

	rootCert, rootKey, err := pki.GenerateRoot(op.ClusterID, pki.DefaultRootTTL)
	if err != nil {
		return fmt.Errorf("generate root: %w", err)
	}

	intCert, intKey, err := pki.GenerateIntermediate(op.ClusterID, rootCert, rootKey, pki.DefaultIntermediateTTL)
	if err != nil {
		return fmt.Errorf("generate intermediate: %w", err)
	}

	rootKeyPEM, err := encodePrivateKeyPEM(rootKey)
	if err != nil {
		return fmt.Errorf("encode root key: %w", err)
	}

	intKeyPEM, err := encodePrivateKeyPEM(intKey)
	if err != nil {
		return fmt.Errorf("encode intermediate key: %w", err)
	}

	if err := s.store.BootstrapPKI(ctx, &db.PKIBootstrap{
		HMACKey: hmacKey,
		Root: &db.ClusterPKIRoot{
			Fingerprint: pki.Fingerprint(rootCert),
			CertPEM:     string(pki.EncodeCertPEM(rootCert)),
			KeyPEM:      rootKeyPEM,
			AddedAt:     time.Now().Unix(),
			Status:      db.PKIStatusActive,
		},
		Intermediate: &db.ClusterPKIIntermediate{
			Fingerprint:     pki.Fingerprint(intCert),
			CertPEM:         string(pki.EncodeCertPEM(intCert)),
			KeyPEM:          intKeyPEM,
			RootFingerprint: pki.Fingerprint(rootCert),
			NotAfter:        intCert.NotAfter.Unix(),
			Status:          db.PKIStatusActive,
		},
	}); err != nil {
		return fmt.Errorf("bootstrap pki: %w", err)
	}

	s.mu.Lock()
	s.rootKey = rootKey
	s.intermediateKey = intKey
	s.intermediateCert = intCert
	s.clusterID = op.ClusterID
	s.mu.Unlock()

	logger.RaftLog.Info("PKI issuer bootstrapped",
		zap.String("clusterID", op.ClusterID),
		zap.String("rootFingerprint", pki.Fingerprint(rootCert)),
		zap.String("intermediateFingerprint", pki.Fingerprint(intCert)))

	return nil
}

func statusesFromRoots(rs []db.ClusterPKIRoot) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Status
	}

	return out
}

func statusesFromIntermediates(is []db.ClusterPKIIntermediate) []string {
	out := make([]string, len(is))
	for i, it := range is {
		out[i] = it.Status
	}

	return out
}

func hasActive(statuses []string) bool {
	for _, s := range statuses {
		if s == db.PKIStatusActive {
			return true
		}
	}

	return false
}

// LoadKeys parses the active root and intermediate signing keys from the
// replicated DB and prepares the in-memory issuer. Called on every
// OnBecameLeader. Returns nil (not an error) if no active rows exist —
// that is the pre-bootstrap state on a fresh leader.
func (s *Service) LoadKeys(ctx context.Context) error {
	roots, err := s.store.ListPKIRoots(ctx)
	if err != nil {
		return fmt.Errorf("list roots: %w", err)
	}

	ints, err := s.store.ListPKIIntermediates(ctx)
	if err != nil {
		return fmt.Errorf("list intermediates: %w", err)
	}

	activeRoot := pickActiveRoot(roots)
	activeInt := pickActiveIntermediate(ints)

	if activeRoot == nil || activeInt == nil {
		logger.RaftLog.Info("PKI issuer: no active root or intermediate in DB, issuer will stay idle until bootstrap")
		return nil
	}

	rootKey, err := parsePrivateKeyPEM(activeRoot.KeyPEM)
	if err != nil {
		return fmt.Errorf("parse root key: %w", err)
	}

	intKey, err := parsePrivateKeyPEM(activeInt.KeyPEM)
	if err != nil {
		return fmt.Errorf("parse intermediate key: %w", err)
	}

	intCert, err := pki.ParseCertPEM([]byte(activeInt.CertPEM))
	if err != nil {
		return fmt.Errorf("parse intermediate cert: %w", err)
	}

	op, err := s.store.GetOperator(ctx)
	if err != nil {
		return fmt.Errorf("get operator: %w", err)
	}

	s.mu.Lock()
	s.rootKey = rootKey
	s.intermediateKey = intKey
	s.intermediateCert = intCert
	s.clusterID = op.ClusterID
	s.mu.Unlock()

	return nil
}

func pickActiveRoot(rs []db.ClusterPKIRoot) *db.ClusterPKIRoot {
	for i := range rs {
		if rs[i].Status == db.PKIStatusActive {
			return &rs[i]
		}
	}

	return nil
}

func pickActiveIntermediate(is []db.ClusterPKIIntermediate) *db.ClusterPKIIntermediate {
	for i := range is {
		if is[i].Status == db.PKIStatusActive {
			return &is[i]
		}
	}

	return nil
}

// UnloadKeys zeros the in-memory keys. Called on OnLostLeadership.
func (s *Service) UnloadKeys() {
	s.mu.Lock()
	s.rootKey = nil
	s.intermediateKey = nil
	s.intermediateCert = nil
	s.mu.Unlock()
}

// Ready reports whether the issuer holds a signing intermediate and can
// service Issue requests.
func (s *Service) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.intermediateKey != nil && s.intermediateCert != nil
}

// ValidateCSR checks the CSR's shape against this cluster's ID and the
// caller-asserted nodeID without touching state. Exposed for handlers
// that want a 4xx surface before invoking Issue (which returns 500 on
// the same validation failure via SignLeaf).
func (s *Service) ValidateCSR(csr *x509.CertificateRequest, nodeID int) error {
	s.mu.RLock()
	clusterID := s.clusterID
	s.mu.RUnlock()

	if clusterID == "" {
		return fmt.Errorf("issuer not ready")
	}

	return pki.ValidateLeafCSR(csr, nodeID, clusterID)
}

// Issue validates csr, allocates a serial, signs a leaf, and records the
// issuance in cluster_issued_certs. Returns the leaf PEM.
func (s *Service) Issue(ctx context.Context, csr *x509.CertificateRequest, nodeID int, ttl time.Duration) ([]byte, error) {
	s.mu.RLock()
	intKey := s.intermediateKey
	intCert := s.intermediateCert
	clusterID := s.clusterID
	s.mu.RUnlock()

	if intKey == nil || intCert == nil {
		return nil, fmt.Errorf("issuer not ready")
	}

	if !s.store.IsLeader() {
		return nil, fmt.Errorf("not leader")
	}

	serial, err := s.store.AllocatePKISerial(ctx)
	if err != nil {
		return nil, fmt.Errorf("allocate serial: %w", err)
	}

	issuer := pki.NewIssuer(intCert, intKey, clusterID)

	leafPEM, err := issuer.SignLeaf(csr, nodeID, uint64(serial), ttl)
	if err != nil {
		return nil, err
	}

	leaf, err := pki.ParseCertPEM(leafPEM)
	if err != nil {
		return nil, fmt.Errorf("parse signed leaf: %w", err)
	}

	if err := s.store.RecordIssuedCert(ctx, &db.ClusterIssuedCert{
		Serial:                  serial,
		NodeID:                  nodeID,
		NotAfter:                leaf.NotAfter.Unix(),
		IntermediateFingerprint: pki.Fingerprint(intCert),
		IssuedAt:                time.Now().Unix(),
	}); err != nil {
		return nil, fmt.Errorf("record issued cert: %w", err)
	}

	return leafPEM, nil
}

// MintJoinToken emits a single-use HMAC token bound to nodeID with the
// given TTL. Writes a row in cluster_join_tokens for replay protection.
func (s *Service) MintJoinToken(ctx context.Context, nodeID int, ttl time.Duration) (tokenStr string, err error) {
	if ttl < pki.DefaultJoinTokenMinTTL || ttl > pki.DefaultJoinTokenMaxTTL {
		return "", fmt.Errorf("join-token ttl %s outside [%s, %s]", ttl, pki.DefaultJoinTokenMinTTL, pki.DefaultJoinTokenMaxTTL)
	}

	if !s.store.IsLeader() {
		return "", fmt.Errorf("not leader")
	}

	state, err := s.store.GetPKIState(ctx)
	if err != nil {
		return "", fmt.Errorf("get pki state: %w", err)
	}

	rootFP, err := s.ActiveRootFingerprint(ctx)
	if err != nil {
		return "", fmt.Errorf("active root fingerprint: %w", err)
	}

	op, err := s.store.GetOperator(ctx)
	if err != nil {
		return "", fmt.Errorf("get operator: %w", err)
	}

	if op.ClusterID == "" {
		return "", fmt.Errorf("cluster id not yet populated")
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
		CAFingerprint: rootFP,
		ClusterID:     op.ClusterID,
	}

	tokenStr, err = pki.MintJoinToken(state.HMACKey, claims)
	if err != nil {
		return "", err
	}

	claimsStr, err := claimsJSON(claims)
	if err != nil {
		return "", fmt.Errorf("encode claims: %w", err)
	}

	if err := s.store.MintJoinTokenRecord(ctx, &db.ClusterJoinToken{
		ID:         tokenID,
		NodeID:     nodeID,
		ClaimsJSON: claimsStr,
		ExpiresAt:  claims.ExpiresAt,
	}); err != nil {
		return "", fmt.Errorf("record join token: %w", err)
	}

	return tokenStr, nil
}

// VerifyAndConsumeJoinToken authenticates tokenStr, enforces expiry and
// single-use semantics, and records consumption. Returns the claims on
// success.
func (s *Service) VerifyAndConsumeJoinToken(ctx context.Context, tokenStr string) (*pki.JoinClaims, error) {
	state, err := s.store.GetPKIState(ctx)
	if err != nil {
		return nil, err
	}

	claims, err := pki.VerifyJoinToken(state.HMACKey, time.Now(), tokenStr)
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

// CurrentBundle reads the current replicated trust bundle. Any voter can
// call this; the leader is not required.
func (s *Service) CurrentBundle(ctx context.Context) (*pki.TrustBundle, error) {
	op, err := s.store.GetOperator(ctx)
	if err != nil {
		return nil, err
	}

	if op.ClusterID == "" {
		return nil, fmt.Errorf("cluster not yet bootstrapped")
	}

	rootRows, err := s.store.ListPKIRoots(ctx)
	if err != nil {
		return nil, err
	}

	intRows, err := s.store.ListPKIIntermediates(ctx)
	if err != nil {
		return nil, err
	}

	bundle := &pki.TrustBundle{ClusterID: op.ClusterID}

	for _, r := range rootRows {
		if r.Status == db.PKIStatusRetired {
			continue
		}

		c, err := pki.ParseCertPEM([]byte(r.CertPEM))
		if err != nil {
			return nil, fmt.Errorf("parse root %s: %w", r.Fingerprint, err)
		}

		bundle.Roots = append(bundle.Roots, c)
	}

	for _, r := range intRows {
		if r.Status == db.PKIStatusRetired {
			continue
		}

		c, err := pki.ParseCertPEM([]byte(r.CertPEM))
		if err != nil {
			return nil, fmt.Errorf("parse intermediate %s: %w", r.Fingerprint, err)
		}

		bundle.Intermediates = append(bundle.Intermediates, c)
	}

	// Fail closed while PKI bootstrap hasn't replicated yet. Without
	// this, a follower that sees the operator row (ClusterID set) but
	// not yet the pki_roots rows would silently cache an empty bundle
	// and reject every incoming mTLS handshake with "trust bundle has
	// no roots" until the next leadership transition.
	if len(bundle.Roots) == 0 {
		return nil, fmt.Errorf("trust bundle has no active roots yet")
	}

	return bundle, nil
}

// ActiveRootFingerprint returns the fingerprint of the single active root
// (there is only one outside the brief rotate-root window).
func (s *Service) ActiveRootFingerprint(ctx context.Context) (string, error) {
	rootRows, err := s.store.ListPKIRoots(ctx)
	if err != nil {
		return "", err
	}

	for _, r := range rootRows {
		if r.Status == db.PKIStatusActive {
			return r.Fingerprint, nil
		}
	}

	return "", fmt.Errorf("no active root")
}

// encodePrivateKeyPEM returns the PKCS#8 PEM encoding of a private key.
// Used only for writes into the replicated DB — callers must not log or
// otherwise emit the returned bytes.
func encodePrivateKeyPEM(key crypto.Signer) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// parsePrivateKeyPEM parses a PKCS#8 PEM private key and asserts the
// resulting key satisfies crypto.Signer.
func parsePrivateKeyPEM(keyPEM []byte) (crypto.Signer, error) {
	if len(keyPEM) == 0 {
		return nil, fmt.Errorf("empty private key PEM")
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("not a PRIVATE KEY PEM")
	}

	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKCS#8: %w", err)
	}

	signer, ok := k.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("key is not a crypto.Signer")
	}

	return signer, nil
}

func claimsJSON(c pki.JoinClaims) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
