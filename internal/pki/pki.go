// Copyright 2026 Ella Networks

// Package pki implements the cluster PKI primitives: certificate signing,
// trust-bundle verification, CSR generation, join-token HMAC, and an
// in-memory revocation cache.
//
// This package has no dependencies on internal/db or hashicorp/raft. It
// operates on plain crypto types and lets higher layers handle persistence
// and replication.
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
	"time"
)

// Leaf and CA template constants.
//
// LeafBackdate matches Consul's 1-minute backdate
// (consul/agent/connect/ca/provider_consul.go:370-391) to tolerate
// sub-minute clock skew between nodes.
const (
	LeafBackdate = time.Minute

	// SpiffeTrustDomain is the trust-domain segment of every Ella-Core
	// cluster leaf's URI SAN. The cluster-id follows.
	SpiffeTrustDomain = "cluster.ella"
)

// TTL defaults. These are package-level constants by design: the deployed
// cluster runs with these values unless a build tag overrides them.
//
// The pki_test_ttls build tag in ttls_test.go replaces these with much
// smaller values so integration tests can exercise rotation in seconds.
var (
	DefaultLeafTTL                    = 24 * time.Hour
	DefaultIntermediateTTL            = 90 * 24 * time.Hour
	DefaultRootTTL                    = 10 * 365 * 24 * time.Hour
	DefaultIntermediateRotationFactor = 0.66
	DefaultJoinTokenMinTTL            = 5 * time.Minute
	DefaultJoinTokenMaxTTL            = 24 * time.Hour
)

// Bounds enforced on operator-visible input.
const (
	MinLeafTTL = time.Hour
	MaxLeafTTL = 7 * 24 * time.Hour
	MinNodeID  = 1
	MaxNodeID  = 63
)

// leafKeyUsage and leafExtKeyUsage are the key-usage bits Ella-Core cluster
// leaves carry. EKU serverAuth+clientAuth lets the same leaf be used for
// inbound accepts and outbound dials over the mTLS cluster listener.
var (
	leafKeyUsage    = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	leafExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
)

// Issuer signs cluster node leaves with an in-memory intermediate key.
// Instances are created on the Raft leader after the intermediate key is
// loaded from disk. Zeroize by dropping the reference on leadership loss.
type Issuer struct {
	intermediateCert *x509.Certificate
	intermediateKey  crypto.Signer
	clusterID        string
}

// NewIssuer constructs an Issuer from the intermediate cert + key. Caller
// is responsible for validating the cert signs with the matching key.
func NewIssuer(intermediateCert *x509.Certificate, intermediateKey crypto.Signer, clusterID string) *Issuer {
	return &Issuer{
		intermediateCert: intermediateCert,
		intermediateKey:  intermediateKey,
		clusterID:        clusterID,
	}
}

// IntermediateCert returns the active intermediate certificate.
func (i *Issuer) IntermediateCert() *x509.Certificate {
	return i.intermediateCert
}

// SignLeaf validates csr and emits a PEM-encoded cluster leaf signed by the
// issuer's intermediate. nodeID must match the CN and URI-SAN node segment
// in csr. serial is allocated by the caller (monotonic through Raft; see
// internal/db.AllocatePKISerial).
func (i *Issuer) SignLeaf(csr *x509.CertificateRequest, nodeID int, serial uint64, ttl time.Duration) ([]byte, error) {
	if err := ValidateLeafCSR(csr, nodeID, i.clusterID); err != nil {
		return nil, err
	}

	if ttl < MinLeafTTL || ttl > MaxLeafTTL {
		return nil, fmt.Errorf("leaf ttl %s outside [%s, %s]", ttl, MinLeafTTL, MaxLeafTTL)
	}

	notBefore := time.Now().Add(-LeafBackdate)
	notAfter := notBefore.Add(ttl + LeafBackdate)

	tmpl := &x509.Certificate{
		SerialNumber:          new(big.Int).SetUint64(serial),
		Subject:               pkix.Name{CommonName: csr.Subject.CommonName},
		URIs:                  csr.URIs,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              leafKeyUsage,
		ExtKeyUsage:           leafExtKeyUsage,
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, i.intermediateCert, csr.PublicKey, i.intermediateKey)
	if err != nil {
		return nil, fmt.Errorf("sign leaf: %w", err)
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), nil
}

// GenerateRoot creates a fresh self-signed root certificate for the
// cluster's trust anchor. Called exactly once in the cluster's lifetime
// (plus once more per manual rotate-root invocation).
func GenerateRoot(clusterID string, ttl time.Duration) (*x509.Certificate, crypto.Signer, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate root key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now().Add(-LeafBackdate)

	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "ella-cluster-root:" + clusterID},
		NotBefore:             now,
		NotAfter:              now.Add(ttl),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		// MaxPathLen unset + MaxPathLenZero=false = unlimited chain
		// length, per crypto/x509 semantics. Needed because cross-signed
		// root rotation can produce a chain root → crossSigned → int →
		// leaf, which is two intermediates under the anchor root.
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("create root cert: %w", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("parse root cert: %w", err)
	}

	return cert, key, nil
}

// GenerateIntermediate creates a fresh intermediate key pair and signs it
// with the provided root. Called at first-leader PKI bootstrap and on
// every intermediate rotation.
func GenerateIntermediate(clusterID string, root *x509.Certificate, rootKey crypto.Signer, ttl time.Duration) (*x509.Certificate, crypto.Signer, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate intermediate key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now().Add(-LeafBackdate)

	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "ella-cluster-int:" + clusterID},
		NotBefore:             now,
		NotAfter:              now.Add(ttl),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           leafExtKeyUsage,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, root, &key.PublicKey, rootKey)
	if err != nil {
		return nil, nil, fmt.Errorf("sign intermediate: %w", err)
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, fmt.Errorf("parse intermediate: %w", err)
	}

	return cert, key, nil
}

// CrossSign signs subject with signerCert/signerKey's private material,
// preserving subject's public key and subject DN. Used during CA rotation
// so leaves chaining through the old intermediate/root remain valid under
// a bundle that also contains the new one.
func CrossSign(subject *x509.Certificate, signerCert *x509.Certificate, signerKey crypto.Signer) (*x509.Certificate, error) {
	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               subject.Subject,
		NotBefore:             subject.NotBefore,
		NotAfter:              subject.NotAfter,
		KeyUsage:              subject.KeyUsage,
		ExtKeyUsage:           subject.ExtKeyUsage,
		BasicConstraintsValid: true,
		IsCA:                  subject.IsCA,
		MaxPathLen:            subject.MaxPathLen,
		MaxPathLenZero:        subject.MaxPathLenZero,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, signerCert, subject.PublicKey, signerKey)
	if err != nil {
		return nil, fmt.Errorf("cross-sign: %w", err)
	}

	return x509.ParseCertificate(der)
}

// Fingerprint returns the SHA-256 fingerprint of cert's DER bytes, prefixed
// with "sha256:". Used to pin roots by hash.
func Fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// SpiffeID formats the URI-SAN identity for a cluster node.
func SpiffeID(clusterID string, nodeID int) *url.URL {
	return &url.URL{
		Scheme: "spiffe",
		Host:   SpiffeTrustDomain,
		Path:   fmt.Sprintf("/%s/node/%d", clusterID, nodeID),
	}
}

// ValidateLeafCSR checks that csr is well-formed for a cluster leaf:
// CN matches the expected node-id, there is exactly one URI SAN matching
// the expected spiffe ID, and no other SANs sneak in.
func ValidateLeafCSR(csr *x509.CertificateRequest, nodeID int, clusterID string) error {
	if csr == nil {
		return fmt.Errorf("csr is nil")
	}

	if nodeID < MinNodeID || nodeID > MaxNodeID {
		return fmt.Errorf("node-id %d outside [%d, %d]", nodeID, MinNodeID, MaxNodeID)
	}

	wantCN := fmt.Sprintf("ella-node-%d", nodeID)
	if csr.Subject.CommonName != wantCN {
		return fmt.Errorf("csr CN %q does not match expected %q", csr.Subject.CommonName, wantCN)
	}

	if len(csr.URIs) != 1 {
		return fmt.Errorf("csr must carry exactly one URI SAN, got %d", len(csr.URIs))
	}

	wantURI := SpiffeID(clusterID, nodeID)
	if csr.URIs[0].String() != wantURI.String() {
		return fmt.Errorf("csr URI SAN %q does not match expected %q", csr.URIs[0], wantURI)
	}

	if len(csr.DNSNames) != 0 || len(csr.IPAddresses) != 0 || len(csr.EmailAddresses) != 0 {
		return fmt.Errorf("csr carries disallowed SANs (DNS/IP/email)")
	}

	return nil
}

func randomSerial() (*big.Int, error) {
	// 128-bit serial is the common conservative choice (RFC 5280 §4.1.2.2
	// only requires 20 octets max, but ≥ 64 bits of entropy is standard).
	upper := new(big.Int).Lsh(big.NewInt(1), 128)

	n, err := rand.Int(rand.Reader, upper)
	if err != nil {
		return nil, fmt.Errorf("random serial: %w", err)
	}

	return n, nil
}
