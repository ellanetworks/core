// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/ellanetworks/core/internal/pki"
	"github.com/ellanetworks/core/internal/roaming/wire"
)

type PartnerConfig struct {
	ID             string
	Name           string
	PLMNs          []wire.PlmnID
	FQDN           string
	N32cURI        string
	N32fURI        string
	TrustAnchorPEM string
	NRFAPIRoot     string
}

type partner struct {
	id      string
	name    string
	fqdn    string
	plmns   []wire.PlmnID
	main    wire.PlmnID
	n32c    APIRoot
	n32f    APIRoot
	nrf     APIRoot
	trust   *trustedPartner
	key     [sha256.Size]byte
	problem string
}

func (p *partner) usable() bool {
	return p.problem == ""
}

func newPartner(cfg PartnerConfig) *partner {
	p := &partner{
		id:    cfg.ID,
		name:  cfg.Name,
		fqdn:  strings.ToLower(cfg.FQDN),
		plmns: slices.Clone(cfg.PLMNs),
		key:   partnerKey(cfg),
	}

	if err := p.build(cfg); err != nil {
		p.problem = err.Error()
	}

	return p
}

func (p *partner) build(cfg PartnerConfig) error {
	name, err := pki.ParseSEPPName(p.fqdn)
	if err != nil {
		return fmt.Errorf("SEPP FQDN: %w", err)
	}

	plmns := make([]pki.PLMN, 0, len(p.plmns))
	found := false

	for _, plmn := range p.plmns {
		pp := pki.PLMN{MCC: plmn.Mcc, MNC: plmn.Mnc}
		plmns = append(plmns, pp)

		if !found && pp.Equivalent(name.PLMN) {
			p.main, found = plmn, true
		}
	}

	if !found {
		return fmt.Errorf("SEPP FQDN %s names none of the partner's PLMN IDs", p.fqdn)
	}

	if p.n32c, err = ParseAPIRoot(cfg.N32cURI); err != nil || p.n32c.Scheme != "https" {
		return fmt.Errorf("N32-c URI %q is not an https apiRoot", cfg.N32cURI)
	}

	p.n32f = p.n32c

	if cfg.N32fURI != "" {
		if p.n32f, err = ParseAPIRoot(cfg.N32fURI); err != nil || p.n32f.Scheme != "https" {
			return fmt.Errorf("N32-f URI %q is not an https apiRoot", cfg.N32fURI)
		}
	}

	p.nrf = APIRoot{Scheme: "https", Authority: p.main.NFFQDN(wire.FQDNLabelNRF)}

	if cfg.NRFAPIRoot != "" {
		if p.nrf, err = ParseAPIRoot(cfg.NRFAPIRoot); err != nil {
			return fmt.Errorf("NRF apiRoot %q: %w", cfg.NRFAPIRoot, err)
		}
	}

	if p.trust, err = newTrustedPartner(p.id, p.fqdn, plmns, cfg.TrustAnchorPEM); err != nil {
		return err
	}

	return nil
}

type seppPeer struct {
	fqdn string
	n32c APIRoot
	n32f APIRoot
}

func (p *partner) defaultPeer() seppPeer {
	return seppPeer{fqdn: p.fqdn, n32c: p.n32c, n32f: p.n32f}
}

func (s seppPeer) exchangeCapabilityURL() string {
	return s.n32c.String() + N32cExchangeCapabilityPath
}

func (p *partner) redirectPeer(location string) (seppPeer, error) {
	if location == "" {
		return seppPeer{}, fmt.Errorf("%w: 307 without Location", ErrRedirect)
	}

	loc, err := url.Parse(location)
	if err != nil || !loc.IsAbs() || !strings.EqualFold(loc.Scheme, "https") || loc.User != nil || loc.RawQuery != "" || loc.Fragment != "" {
		return seppPeer{}, fmt.Errorf("%w: Location %q is not an https URI", ErrRedirect, location)
	}

	prefix := strings.TrimSuffix(strings.TrimSuffix(loc.EscapedPath(), "/"), N32cExchangeCapabilityPath)

	root, err := ParseAPIRoot("https://" + loc.Host + prefix)
	if err != nil {
		return seppPeer{}, fmt.Errorf("%w: Location %q: %w", ErrRedirect, location, err)
	}

	host := strings.TrimSuffix(root.Hostname(), ".")
	if !p.inNetwork(host) {
		return seppPeer{}, fmt.Errorf("%w: Location %q is outside the network of partner %s", ErrRedirect, location, p.name)
	}

	return seppPeer{fqdn: host, n32c: root, n32f: root}, nil
}

func (p *partner) inNetwork(host string) bool {
	name, ok := wire.ParseNetworkFQDN(host)
	if !ok {
		return false
	}

	return slices.ContainsFunc(p.plmns, name.InPLMN)
}

func (p *partner) allowsTarget(target *url.URL) error {
	if target == nil {
		return fmt.Errorf("%w: no target URI", ErrTargetNotAllowed)
	}

	host := strings.ToLower(strings.TrimSuffix(target.Hostname(), "."))
	if host == p.nrf.Hostname() || p.inNetwork(host) {
		return nil
	}

	return fmt.Errorf("%w: %s is outside the network of partner %s", ErrTargetNotAllowed, host, p.name)
}

func partnerKey(cfg PartnerConfig) [sha256.Size]byte {
	h := sha256.New()

	for _, plmn := range cfg.PLMNs {
		h.Write([]byte(plmn.String() + "\x00"))
	}

	for _, v := range []string{strings.ToLower(cfg.FQDN), cfg.N32cURI, cfg.N32fURI, cfg.TrustAnchorPEM, cfg.NRFAPIRoot} {
		h.Write([]byte(v + "\x00"))
	}

	var out [sha256.Size]byte

	copy(out[:], h.Sum(nil))

	return out
}
