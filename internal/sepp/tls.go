// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/pki"
)

const ALPNH2 = "h2"

const (
	TLSReasonNoCertificate   = "no_certificate"
	TLSReasonUnknownAnchor   = "unknown_anchor"
	TLSReasonSANMismatch     = "san_mismatch"
	TLSReasonPLMNMismatch    = "plmn_mismatch"
	TLSReasonExpired         = "expired"
	TLSReasonAmbiguous       = "ambiguous_partner"
	TLSReasonALPN            = "alpn"
	TLSReasonNoLocalIdentity = "no_local_identity"
)

var ErrPeerCertificate = errors.New("sepp: peer certificate rejected")

type PeerCertError struct {
	Reason string
	Detail string
}

func (e *PeerCertError) Error() string {
	return ErrPeerCertificate.Error() + ": " + e.Detail
}

func (e *PeerCertError) Unwrap() error {
	return ErrPeerCertificate
}

func peerCertError(reason, format string, args ...any) error {
	return &PeerCertError{Reason: reason, Detail: fmt.Sprintf(format, args...)}
}

func TLSFailureReason(err error) string {
	var pe *PeerCertError
	if errors.As(err, &pe) {
		return pe.Reason
	}

	return ""
}

var tls12Suites = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
}

var curves = []tls.CurveID{tls.X25519, tls.CurveP256, tls.CurveP384}

func baseTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:       tls.VersionTLS12,
		MaxVersion:       tls.VersionTLS13,
		CipherSuites:     slices.Clone(tls12Suites),
		CurvePreferences: slices.Clone(curves),
		NextProtos:       []string{ALPNH2},
	}
}

type trustedPartner struct {
	id    string
	fqdn  string
	plmns []pki.PLMN
	roots *x509.CertPool
}

func newTrustedPartner(id, fqdn string, plmns []pki.PLMN, anchorPEM string) (*trustedPartner, error) {
	anchors, err := pki.ParseCertChainPEM([]byte(anchorPEM))
	if err != nil {
		return nil, fmt.Errorf("trust anchor: %w", err)
	}

	roots := x509.NewCertPool()
	for _, a := range anchors {
		roots.AddCert(a)
	}

	return &trustedPartner{id: id, fqdn: strings.ToLower(fqdn), plmns: plmns, roots: roots}, nil
}

func (p *trustedPartner) check(certs []*x509.Certificate, now time.Time, usage x509.ExtKeyUsage) error {
	return p.checkAs(certs, now, usage, "")
}

func (p *trustedPartner) checkAs(certs []*x509.Certificate, now time.Time, usage x509.ExtKeyUsage, fqdn string) error {
	if len(certs) == 0 {
		return peerCertError(TLSReasonNoCertificate, "no certificate presented")
	}

	leaf := certs[0]

	intermediates := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}

	_, err := leaf.Verify(x509.VerifyOptions{
		Roots:         p.roots,
		Intermediates: intermediates,
		CurrentTime:   now,
		KeyUsages:     []x509.ExtKeyUsage{usage},
	})
	if err != nil {
		var invalid x509.CertificateInvalidError
		if errors.As(err, &invalid) && invalid.Reason == x509.Expired {
			return peerCertError(TLSReasonExpired, "certificate %q is expired or not yet valid: %v", leaf.Subject.CommonName, err)
		}

		return peerCertError(TLSReasonUnknownAnchor, "certificate %q does not chain to the trust anchor of the partner SEPP %s: %v", leaf.Subject.CommonName, p.fqdn, err)
	}

	if fqdn != "" && !slices.ContainsFunc(leaf.DNSNames, func(n string) bool { return sameFQDN(n, fqdn) }) {
		return peerCertError(TLSReasonSANMismatch, "certificate subjectAltName %v does not contain the partner SEPP FQDN %s", leaf.DNSNames, fqdn)
	}

	named := 0

	for _, name := range leaf.DNSNames {
		plmn, ok := pki.PLMNOfFQDN(name)
		if !ok {
			continue
		}

		if !slices.ContainsFunc(p.plmns, plmn.Equivalent) {
			return peerCertError(TLSReasonPLMNMismatch, "certificate subjectAltName %s names PLMN %s-%s, which is not one of the partner's", name, plmn.MCC, plmn.MNC)
		}

		named++
	}

	if named == 0 {
		return peerCertError(TLSReasonPLMNMismatch, "certificate subjectAltName %v names no PLMN", leaf.DNSNames)
	}

	return nil
}

func identifyPeer(partners []*trustedPartner, certs []*x509.Certificate, now time.Time, usage x509.ExtKeyUsage) (*trustedPartner, error) {
	if len(certs) == 0 {
		return nil, peerCertError(TLSReasonNoCertificate, "no client certificate presented")
	}

	var (
		matches []*trustedPartner
		best    error
	)

	for _, p := range partners {
		err := p.check(certs, now, usage)
		if err == nil {
			matches = append(matches, p)
			continue
		}

		if best == nil || reasonRank(TLSFailureReason(err)) > reasonRank(TLSFailureReason(best)) {
			best = err
		}
	}

	switch {
	case len(matches) == 1:
		return matches[0], nil
	case len(matches) > 1:
		ids := make([]string, 0, len(matches))
		for _, m := range matches {
			ids = append(ids, m.fqdn)
		}

		return nil, peerCertError(TLSReasonAmbiguous, "certificate matches several partner SEPPs: %s", strings.Join(ids, ", "))
	case best != nil:
		return nil, best
	}

	return nil, peerCertError(TLSReasonUnknownAnchor, "no roaming partner has a 5G agreement")
}

func reasonRank(reason string) int {
	switch reason {
	case TLSReasonPLMNMismatch:
		return 4
	case TLSReasonSANMismatch:
		return 3
	case TLSReasonExpired:
		return 2
	case TLSReasonUnknownAnchor:
		return 1
	}

	return 0
}

func checkALPN(cs tls.ConnectionState) error {
	if cs.NegotiatedProtocol != ALPNH2 {
		return peerCertError(TLSReasonALPN, "ALPN %q negotiated, only h2 is allowed", cs.NegotiatedProtocol)
	}

	return nil
}

func ServerName(n32Host, peerFQDN string) string {
	if n32Host == "" || net.ParseIP(strings.Trim(n32Host, "[]")) != nil {
		return peerFQDN
	}

	return n32Host
}
