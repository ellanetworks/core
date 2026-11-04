// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/pki"
	"github.com/ellanetworks/core/internal/roaming/wire"
)

type IdentityConfig struct {
	SEPPID       int
	FQDN         string
	CertChainPEM string
	KeyPEM       string
	Generated    bool
}

type localIdentity struct {
	cfg       IdentityConfig
	seppID    int
	fqdn      string
	cert      *tls.Certificate
	leaf      *x509.Certificate
	anchor    *x509.Certificate
	generated bool
}

var ErrNoIdentity = errors.New("sepp: no SEPP identity")

func newLocalIdentity(cfg IdentityConfig, op wire.PlmnID) (*localIdentity, error) {
	parsed, err := pki.ValidateSEPPIdentity([]byte(cfg.CertChainPEM), []byte(cfg.KeyPEM), cfg.FQDN)
	if err != nil {
		return nil, err
	}

	if !strings.HasSuffix(strings.ToLower(cfg.FQDN), "."+pki.PLMN{MCC: op.Mcc, MNC: op.Mnc}.Domain()) {
		return nil, fmt.Errorf("SEPP FQDN %s is not under the home PLMN domain %s", cfg.FQDN, pki.PLMN{MCC: op.Mcc, MNC: op.Mnc}.Domain())
	}

	cert, err := tls.X509KeyPair([]byte(cfg.CertChainPEM), []byte(cfg.KeyPEM))
	if err != nil {
		return nil, err
	}

	return &localIdentity{
		cfg:       cfg,
		seppID:    cfg.SEPPID,
		fqdn:      strings.ToLower(cfg.FQDN),
		cert:      &cert,
		leaf:      parsed.Leaf(),
		anchor:    parsed.TrustAnchor(),
		generated: cfg.Generated,
	}, nil
}

func (id *localIdentity) sameSession(o *localIdentity) bool {
	if id == nil || o == nil {
		return id == o
	}

	return id.seppID == o.seppID && id.fqdn == o.fqdn && id.anchor.Equal(o.anchor)
}
