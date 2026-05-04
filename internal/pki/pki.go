// Copyright 2026 Ella Networks

// Package pki implements the cluster-TLS primitives used by every node:
//
//   - GenerateNodeCert: a per-node ECDSA P-256 self-signed cert with a
//     long validity (10y by default). Each node owns its key; the cert
//     is published cluster-wide via Raft and trust is established by
//     SHA-256 pinning, not chain validation.
//   - Fingerprint: the SHA-256 (hex, "sha256:" prefixed) of a cert's
//     DER bytes. The pinning unit.
//   - SpiffeID and identity helpers: the URI SAN format
//     spiffe://cluster.ella/<clusterID>/node/<n> is preserved from the
//     legacy chain-based design, since both the listener verifier and
//     the join-flow CSR-style messages need to extract (clusterID,
//     nodeID) from a peer cert.
//   - Join-token primitives: see tokens.go.
//
// This package has no dependencies on internal/db or hashicorp/raft.
package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// SpiffeTrustDomain is the trust-domain segment of every Ella-Core
	// cluster cert's URI SAN. The cluster-id follows.
	SpiffeTrustDomain = "cluster.ella"

	// DefaultNodeCertTTL is how long a freshly generated self-signed
	// cluster cert is valid. 10 years is "effectively forever" — it
	// outlives the binary's deployment lifecycle and removes expiry
	// from the cluster-liveness path. Optional rotation re-pins long
	// before this matters.
	DefaultNodeCertTTL = 10 * 365 * 24 * time.Hour
)

// Bounds enforced on operator-visible input.
const (
	MinNodeID = 1
	MaxNodeID = 63

	DefaultJoinTokenMinTTL = 5 * time.Minute
	DefaultJoinTokenMaxTTL = 24 * time.Hour
)

// nodeCertKeyUsage and nodeCertExtKeyUsage are the key-usage bits a
// cluster node cert carries. EKU serverAuth+clientAuth lets the same
// cert be used for inbound accepts and outbound dials over the mTLS
// cluster listener.
var (
	nodeCertKeyUsage    = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	nodeCertExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
)

// SpiffeID builds the URI SAN that every cluster cert carries.
func SpiffeID(clusterID string, nodeID int) *url.URL {
	return &url.URL{
		Scheme: "spiffe",
		Host:   SpiffeTrustDomain,
		Path:   fmt.Sprintf("/%s/node/%d", clusterID, nodeID),
	}
}

// GenerateNodeCert produces a fresh ECDSA P-256 self-signed certificate
// for nodeID under clusterID, valid for ttl. Returns the parsed cert
// and the signer; PEM-encode via EncodeCertPEM / EncodePrivateKeyPEM.
func GenerateNodeCert(nodeID int, clusterID string, ttl time.Duration) (*x509.Certificate, crypto.Signer, error) {
	if nodeID < MinNodeID || nodeID > MaxNodeID {
		return nil, nil, fmt.Errorf("node-id %d outside [%d, %d]", nodeID, MinNodeID, MaxNodeID)
	}

	if clusterID == "" {
		return nil, nil, fmt.Errorf("clusterID must not be empty")
	}

	if ttl <= 0 {
		ttl = DefaultNodeCertTTL
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate node key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}

	skid, err := subjectKeyID(&key.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	now := time.Now().Add(-time.Minute) // small backdate for clock skew

	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: fmt.Sprintf("ella-node-%d", nodeID)},
		URIs:                  []*url.URL{SpiffeID(clusterID, nodeID)},
		NotBefore:             now,
		NotAfter:              now.Add(ttl + time.Minute),
		KeyUsage:              nodeCertKeyUsage,
		ExtKeyUsage:           nodeCertExtKeyUsage,
		BasicConstraintsValid: true,
		IsCA:                  false,
		SubjectKeyId:          skid,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create self-signed cert: %w", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("parse self-signed cert: %w", err)
	}

	return cert, key, nil
}

// Fingerprint returns the canonical "sha256:<hex>" pin for a cert.
func Fingerprint(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}

	sum := sha256.Sum256(cert.Raw)

	return "sha256:" + hex.EncodeToString(sum[:])
}

// FingerprintRaw returns the 32-byte SHA-256 of a cert's DER bytes,
// without the textual prefix. Used by the bootstrap dialer's
// VerifyConnection callback for constant-time comparison.
func FingerprintRaw(cert *x509.Certificate) []byte {
	if cert == nil {
		return nil
	}

	sum := sha256.Sum256(cert.Raw)

	return sum[:]
}

// ParseFingerprint decodes a "sha256:<hex>" string into raw bytes.
func ParseFingerprint(s string) ([]byte, error) {
	s = strings.TrimPrefix(s, "sha256:")

	raw, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("decode fingerprint hex: %w", err)
	}

	if len(raw) != sha256.Size {
		return nil, fmt.Errorf("fingerprint length %d, want %d", len(raw), sha256.Size)
	}

	return raw, nil
}

// ParseCertPEM decodes a PEM-encoded certificate.
func ParseCertPEM(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("not a CERTIFICATE PEM block")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse certificate: %w", err)
	}

	return cert, nil
}

// EncodeCertPEM returns the PEM encoding of cert. Returns nil for a nil
// cert rather than panicking.
func EncodeCertPEM(cert *x509.Certificate) []byte {
	if cert == nil {
		return nil
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

// EncodePrivateKeyPEM marshals a key as PKCS#8 PEM.
func EncodePrivateKeyPEM(key crypto.Signer) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// ParsePrivateKeyPEM decodes a PKCS#8 PEM private key as a crypto.Signer.
func ParsePrivateKeyPEM(keyPEM []byte) (crypto.Signer, error) {
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

// IdentityFromCert validates the SPIFFE URI SAN of a cluster cert and
// returns (clusterID, nodeID). Used by the join-flow handler to confirm
// that a self-signed cert presented for registration matches the
// claims in its associated join token.
func IdentityFromCert(cert *x509.Certificate) (clusterID string, nodeID int, err error) {
	if cert == nil {
		return "", 0, fmt.Errorf("nil cert")
	}

	if len(cert.URIs) != 1 {
		return "", 0, fmt.Errorf("cluster cert must carry exactly one URI SAN, got %d", len(cert.URIs))
	}

	u := cert.URIs[0]
	if u.Scheme != "spiffe" || u.Host != SpiffeTrustDomain {
		return "", 0, fmt.Errorf("URI SAN %q is not a valid SPIFFE URI for %s", u, SpiffeTrustDomain)
	}

	path := strings.TrimPrefix(u.Path, "/")

	cid, rest, ok := strings.Cut(path, "/")
	if !ok || cid == "" || !strings.HasPrefix(rest, "node/") {
		return "", 0, fmt.Errorf("URI SAN path %q is not in the form /<clusterID>/node/<n>", u.Path)
	}

	suffix := rest[len("node/"):]
	if suffix == "" {
		return "", 0, fmt.Errorf("URI SAN path %q has empty node segment", u.Path)
	}

	// Canonical unsigned decimal — reject "+5", "-5", "05".
	n, err := strconv.ParseUint(suffix, 10, 31)
	if err != nil {
		return "", 0, fmt.Errorf("URI SAN node segment %q: %w", suffix, err)
	}

	if strconv.FormatUint(n, 10) != suffix {
		return "", 0, fmt.Errorf("URI SAN node segment %q is not in canonical form", suffix)
	}

	id := int(n)
	if id < MinNodeID || id > MaxNodeID {
		return "", 0, fmt.Errorf("URI SAN node-id %d outside [%d, %d]", id, MinNodeID, MaxNodeID)
	}

	wantCN := fmt.Sprintf("ella-node-%d", id)
	if cert.Subject.CommonName != wantCN {
		return "", 0, fmt.Errorf("cert CN %q does not match URI-SAN node %d", cert.Subject.CommonName, id)
	}

	return cid, id, nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 159)

	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("random serial: %w", err)
	}

	return n, nil
}

// subjectKeyID computes RFC 5280 §4.2.1.2 SKID from a public key.
func subjectKeyID(pub crypto.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("marshal pubkey: %w", err)
	}

	// The SKID per RFC 5280 §4.2.1.2 (method 1) is the SHA-1 of the
	// BIT STRING subjectPublicKey component (without tag/length).
	// Stdlib does this for ParsePKIXPublicKey output via SHA-1 of the
	// full DER; that's also accepted in practice.
	sum := sha256.Sum256(der)

	return sum[:20], nil
}
