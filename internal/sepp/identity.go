// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"strconv"
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
	seppID    int
	fqdn      string
	cert      *tls.Certificate
	leaf      *x509.Certificate
	anchor    *x509.Certificate
	generated bool
	key       [sha256.Size]byte
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
		seppID:    cfg.SEPPID,
		fqdn:      strings.ToLower(cfg.FQDN),
		cert:      &cert,
		leaf:      parsed.Leaf(),
		anchor:    parsed.TrustAnchor(),
		generated: cfg.Generated,
		key:       identityKey(cfg),
	}, nil
}

func identityKey(cfg IdentityConfig) [sha256.Size]byte {
	return sha256.Sum256([]byte(strconv.Itoa(cfg.SEPPID) + "\x00" + strconv.FormatBool(cfg.Generated) + "\x00" + cfg.FQDN + "\x00" + cfg.CertChainPEM + "\x00" + cfg.KeyPEM))
}
