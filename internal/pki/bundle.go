// Copyright 2026 Ella Networks

package pki

import (
	"crypto/x509"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// TrustBundle is the set of roots and intermediates the cluster currently
// trusts, plus the local cluster-id used to reject cross-cluster leaves.
// Built from replicated state on every voter; the leader mutates by
// proposing Raft commands, followers observe.
type TrustBundle struct {
	// Roots and Intermediates include both "active" and "verify-only"
	// certs: callers distinguish if they need to pick a cert for signing
	// (use ActiveIntermediate), but chain verification considers every
	// entry trusted.
	Roots         []*x509.Certificate
	Intermediates []*x509.Certificate
	ClusterID     string
}

// Verify chain-verifies cert against the bundle's roots+intermediates,
// then enforces cluster-local rules:
//
//  1. URI SAN starts with "spiffe://cluster.ella/<clusterID>/node/"
//     (cross-cluster rejection; closes audit F-10).
//  2. URI SAN node-id parses and is in the [MinNodeID, MaxNodeID] range.
//  3. CN matches "ella-node-<n>" with the same n.
//
// Returns the extracted nodeID on success.
func (b *TrustBundle) Verify(cert *x509.Certificate, now time.Time) (int, error) {
	if b == nil {
		return 0, fmt.Errorf("nil trust bundle")
	}

	if len(b.Roots) == 0 {
		return 0, fmt.Errorf("trust bundle has no roots")
	}

	roots := x509.NewCertPool()
	for _, r := range b.Roots {
		roots.AddCert(r)
	}

	intermediates := x509.NewCertPool()
	for _, ic := range b.Intermediates {
		intermediates.AddCert(ic)
	}

	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: intermediates,
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		CurrentTime:   now,
	}); err != nil {
		return 0, fmt.Errorf("chain verification failed: %w", err)
	}

	nodeID, err := identityFromURISAN(cert, b.ClusterID)
	if err != nil {
		return 0, err
	}

	wantCN := fmt.Sprintf("ella-node-%d", nodeID)
	if cert.Subject.CommonName != wantCN {
		return 0, fmt.Errorf("leaf CN %q does not match URI-SAN node %d", cert.Subject.CommonName, nodeID)
	}

	return nodeID, nil
}

// identityFromURISAN enforces the spiffe URI shape and returns the nodeID.
func identityFromURISAN(cert *x509.Certificate, clusterID string) (int, error) {
	if len(cert.URIs) != 1 {
		return 0, fmt.Errorf("leaf must carry exactly one URI SAN, got %d", len(cert.URIs))
	}

	u := cert.URIs[0]

	if u.Scheme != "spiffe" {
		return 0, fmt.Errorf("leaf URI SAN scheme %q is not spiffe", u.Scheme)
	}

	if u.Host != SpiffeTrustDomain {
		return 0, fmt.Errorf("leaf URI SAN trust-domain %q is not %q", u.Host, SpiffeTrustDomain)
	}

	// Path format: "/<clusterID>/node/<n>".
	wantPrefix := "/" + clusterID + "/node/"
	if !strings.HasPrefix(u.Path, wantPrefix) {
		return 0, fmt.Errorf("leaf URI SAN path %q does not start with %q", u.Path, wantPrefix)
	}

	suffix := u.Path[len(wantPrefix):]
	if suffix == "" {
		return 0, fmt.Errorf("leaf URI SAN path %q has empty node segment", u.Path)
	}

	// Parse the node-id segment as an integer.
	n := new(big.Int)
	if _, ok := n.SetString(suffix, 10); !ok {
		return 0, fmt.Errorf("leaf URI SAN node segment %q is not an integer", suffix)
	}

	if !n.IsInt64() {
		return 0, fmt.Errorf("leaf URI SAN node segment %q out of range", suffix)
	}

	id := int(n.Int64())
	if id < MinNodeID || id > MaxNodeID {
		return 0, fmt.Errorf("leaf URI SAN node-id %d outside [%d, %d]", id, MinNodeID, MaxNodeID)
	}

	return id, nil
}
