// Copyright 2026 Ella Networks

// Package pkiissuer is the cluster's PKI issuer service. It runs on the
// Raft leader: generates root+intermediate on first boot, signs leaves
// requested by joining or renewing nodes, mints and verifies join
// tokens, and drives CA rotation.
//
// Private keys live only on voter disks under <dataDir>/cluster-tls/.
// Public material (root cert, active intermediate cert, issued-cert
// tracking, revocation rows, join-token rows, and the hmacKey used to
// authenticate tokens) is replicated through Raft via internal/db.
package pkiissuer

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	InitializePKIState(ctx context.Context, hmacKey []byte) error
	AllocatePKISerial(ctx context.Context) (int64, error)

	ListPKIRoots(ctx context.Context) ([]db.ClusterPKIRoot, error)
	InsertPKIRoot(ctx context.Context, r *db.ClusterPKIRoot) error
	ListPKIIntermediates(ctx context.Context) ([]db.ClusterPKIIntermediate, error)
	InsertPKIIntermediate(ctx context.Context, r *db.ClusterPKIIntermediate) error

	RecordIssuedCert(ctx context.Context, r *db.ClusterIssuedCert) error
	ListRevokedCerts(ctx context.Context) ([]db.ClusterRevokedCert, error)

	MintJoinTokenRecord(ctx context.Context, r *db.ClusterJoinToken) error
	GetJoinToken(ctx context.Context, id string) (*db.ClusterJoinToken, error)
	ConsumeJoinToken(ctx context.Context, id string, nodeID int) error

	IsLeader() bool
}

// Service is the leader-side PKI issuer.
type Service struct {
	store   Store
	dataDir string

	mu               sync.RWMutex
	rootKey          crypto.Signer
	intermediateKey  crypto.Signer
	intermediateCert *x509.Certificate
	clusterID        string
}

// New returns an unloaded Service. Call Bootstrap on first-leader boot
// (from PostInitClusterSetup) and LoadKeys on every OnBecameLeader.
func New(store Store, dataDir string) *Service {
	return &Service{store: store, dataDir: dataDir}
}

// Bootstrap generates root + intermediate + hmac key on the first leader
// to run against a fresh cluster. Idempotent and self-healing: if a
// previous invocation crashed between writes (e.g. hmacKey landed but
// root did not), a retry completes the missing steps against the
// existing pki state instead of returning early.
func (s *Service) Bootstrap(ctx context.Context) error {
	op, err := s.store.GetOperator(ctx)
	if err != nil {
		return fmt.Errorf("get operator: %w", err)
	}

	if op.ClusterID == "" {
		return fmt.Errorf("cluster id not yet populated; run PostInitClusterSetup first")
	}

	state, err := s.store.GetPKIState(ctx)

	switch {
	case err == nil:
		// State row present — this may be a complete bootstrap (happy
		// path, return early) or a partial one (hmacKey persisted,
		// root/intermediate missing). Fall through to the activity
		// checks below.
	case errors.Is(err, db.ErrNotFound):
		hmacKey, hmacErr := pki.NewHMACKey()
		if hmacErr != nil {
			return fmt.Errorf("hmac key: %w", hmacErr)
		}

		if err := s.store.InitializePKIState(ctx, hmacKey); err != nil {
			return fmt.Errorf("init pki state: %w", err)
		}

		state = &db.ClusterPKIState{HMACKey: hmacKey}
	default:
		return fmt.Errorf("check pki state: %w", err)
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
		// Fully bootstrapped already.
		return nil
	}

	rootCert, rootKey, err := pki.GenerateRoot(op.ClusterID, pki.DefaultRootTTL)
	if err != nil {
		return fmt.Errorf("generate root: %w", err)
	}

	intCert, intKey, err := pki.GenerateIntermediate(op.ClusterID, rootCert, rootKey, pki.DefaultIntermediateTTL)
	if err != nil {
		return fmt.Errorf("generate intermediate: %w", err)
	}

	now := time.Now().Unix()

	if err := s.store.InsertPKIRoot(ctx, &db.ClusterPKIRoot{
		Fingerprint: pki.Fingerprint(rootCert),
		CertPEM:     string(pki.EncodeCertPEM(rootCert)),
		AddedAt:     now,
		Status:      db.PKIStatusActive,
	}); err != nil {
		return fmt.Errorf("insert root: %w", err)
	}

	if err := s.store.InsertPKIIntermediate(ctx, &db.ClusterPKIIntermediate{
		Fingerprint:     pki.Fingerprint(intCert),
		CertPEM:         string(pki.EncodeCertPEM(intCert)),
		RootFingerprint: pki.Fingerprint(rootCert),
		NotAfter:        intCert.NotAfter.Unix(),
		Status:          db.PKIStatusActive,
	}); err != nil {
		return fmt.Errorf("insert intermediate: %w", err)
	}

	if err := s.writeKeyToDisk("root.key", rootKey); err != nil {
		return err
	}

	if err := s.writeKeyToDisk("intermediate.key", intKey); err != nil {
		return err
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
		zap.String("intermediateFingerprint", pki.Fingerprint(intCert)),
		zap.Int("hmacKeyBytes", len(state.HMACKey)))

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

// LoadKeys reads root.key + intermediate.key from disk and prepares the
// in-memory issuer. Called on every OnBecameLeader. Returns nil without
// loading if the key files are absent (first-boot pre-bootstrap or
// promoted voter awaiting key transfer).
func (s *Service) LoadKeys(ctx context.Context) error {
	rootKey, err := s.readKeyFromDisk("root.key")
	if err != nil {
		if os.IsNotExist(err) {
			logger.RaftLog.Info("PKI issuer: root.key absent, issuer degraded until bootstrap or key transfer")
			return nil
		}

		return fmt.Errorf("read root.key: %w", err)
	}

	intKey, err := s.readKeyFromDisk("intermediate.key")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return fmt.Errorf("read intermediate.key: %w", err)
	}

	// Resolve the active intermediate cert from replicated state.
	intCerts, err := s.store.ListPKIIntermediates(ctx)
	if err != nil {
		return fmt.Errorf("list intermediates: %w", err)
	}

	var active *x509.Certificate

	for _, r := range intCerts {
		if r.Status != db.PKIStatusActive {
			continue
		}

		c, err := pki.ParseCertPEM([]byte(r.CertPEM))
		if err != nil {
			return fmt.Errorf("parse intermediate %s: %w", r.Fingerprint, err)
		}

		active = c

		break
	}

	if active == nil {
		logger.RaftLog.Warn("PKI issuer: no active intermediate in replicated state despite having key on disk")
		return nil
	}

	op, err := s.store.GetOperator(ctx)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.rootKey = rootKey
	s.intermediateKey = intKey
	s.intermediateCert = active
	s.clusterID = op.ClusterID
	s.mu.Unlock()

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

	// Check the replicated record: already consumed ⇒ reject.
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

// writeKeyToDisk PEM-encodes a private key and writes it under the
// cluster-tls subdirectory with 0600 perms. The temp file is fsync'd
// before rename and the parent directory is fsync'd after, so a crash
// between signalling success and the OS flushing buffers leaves either
// the old key or the new key in place — never a truncated file.
func (s *Service) writeKeyToDisk(name string, key crypto.Signer) error {
	dir := filepath.Join(s.dataDir, db.ClusterTLSDir)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}

	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})

	tmpPath := filepath.Join(dir, name+".tmp")
	finalPath := filepath.Join(dir, name)

	if err := writeFileSync(tmpPath, pemBytes, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %s: %w", finalPath, err)
	}

	if err := syncDir(dir); err != nil {
		return fmt.Errorf("sync dir %s: %w", dir, err)
	}

	return nil
}

// writeFileSync writes data to path with perm, fsync'ing before close so
// the content is durable when the file descriptor is released. Callers
// still need to sync the parent directory to make the rename durable.
func writeFileSync(path string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm) // #nosec G304 -- caller-controlled path under data dir
	if err != nil {
		return err
	}

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}

	return f.Close()
}

func syncDir(dir string) error {
	d, err := os.Open(dir) // #nosec G304 -- caller-controlled path under data dir
	if err != nil {
		return err
	}

	if err := d.Sync(); err != nil {
		_ = d.Close()
		return err
	}

	return d.Close()
}

func (s *Service) readKeyFromDisk(name string) (crypto.Signer, error) {
	path := filepath.Join(s.dataDir, db.ClusterTLSDir, name)

	raw, err := os.ReadFile(path) // #nosec: G304 -- path is under our data directory
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("%s: not a PRIVATE KEY PEM", path)
	}

	k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%s: parse key: %w", path, err)
	}

	signer, ok := k.(crypto.Signer)
	if !ok {
		return nil, fmt.Errorf("%s: key is not a crypto.Signer", path)
	}

	return signer, nil
}

// ExportKeys reads root.key and intermediate.key from disk and returns
// their PEM bytes. Used by the /cluster/pki/keys handler to transfer
// signing material to newly-promoted voters that lack it. Returns
// os.ErrNotExist wrapped in the error if this node has no keys on
// disk (i.e. it was not the bootstrap voter and has not yet received
// a transfer).
func (s *Service) ExportKeys() (rootKeyPEM, intermediateKeyPEM []byte, err error) {
	rootKeyPEM, err = s.readKeyPEMFromDisk("root.key")
	if err != nil {
		return nil, nil, fmt.Errorf("read root.key: %w", err)
	}

	intermediateKeyPEM, err = s.readKeyPEMFromDisk("intermediate.key")
	if err != nil {
		return nil, nil, fmt.Errorf("read intermediate.key: %w", err)
	}

	return rootKeyPEM, intermediateKeyPEM, nil
}

// ImportKeys writes rootKeyPEM and intermediateKeyPEM atomically to disk.
// The caller is responsible for validating the PEM contents parse as
// keys and chain-match the replicated root+intermediate certs before
// invoking Import; this method is the raw persistence primitive.
func (s *Service) ImportKeys(rootKeyPEM, intermediateKeyPEM []byte) error {
	if err := s.writeKeyPEMToDisk("root.key", rootKeyPEM); err != nil {
		return err
	}

	return s.writeKeyPEMToDisk("intermediate.key", intermediateKeyPEM)
}

// HaveKeysOnDisk reports whether root.key and intermediate.key both
// exist under the cluster-tls directory. Used by the key-transfer
// worker to short-circuit once a transfer has completed.
func (s *Service) HaveKeysOnDisk() bool {
	for _, name := range []string{"root.key", "intermediate.key"} {
		if _, err := os.Stat(filepath.Join(s.dataDir, db.ClusterTLSDir, name)); err != nil {
			return false
		}
	}

	return true
}

func (s *Service) readKeyPEMFromDisk(name string) ([]byte, error) {
	path := filepath.Join(s.dataDir, db.ClusterTLSDir, name)

	return os.ReadFile(path) // #nosec: G304 -- path is under our data directory
}

func (s *Service) writeKeyPEMToDisk(name string, pemBytes []byte) error {
	dir := filepath.Join(s.dataDir, db.ClusterTLSDir)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	// Validate it is a PKCS#8 PRIVATE KEY PEM before committing to disk,
	// so a peer returning garbage cannot corrupt our on-disk state.
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "PRIVATE KEY" {
		return fmt.Errorf("%s: not a PRIVATE KEY PEM", name)
	}

	if _, err := x509.ParsePKCS8PrivateKey(block.Bytes); err != nil {
		return fmt.Errorf("%s: parse key: %w", name, err)
	}

	tmpPath := filepath.Join(dir, name+".tmp")
	finalPath := filepath.Join(dir, name)

	if err := writeFileSync(tmpPath, pemBytes, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %s: %w", finalPath, err)
	}

	if err := syncDir(dir); err != nil {
		return fmt.Errorf("sync dir %s: %w", dir, err)
	}

	return nil
}

func claimsJSON(c pki.JoinClaims) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
