// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/ellanetworks/core/internal/db"
)

const (
	CallbackPath       = "/api/v1/auth/oidc/callback"
	PostLogoutPath     = "/login"
	MinSessionLifetime = 5 * time.Minute
	MaxSessionLifetime = 30 * 24 * time.Hour
	MaxScopes          = 20
	MaxGroupRoles      = 100
	maxDisplayName     = 64
	maxValue           = 255
	maxURL             = 2048
	maxCACertificate   = 64 * 1024
)

var ErrInvalid = errors.New("invalid OIDC settings")

type Store interface {
	GetOIDC(ctx context.Context) (db.OIDC, error)
	UpdateOIDC(ctx context.Context, o db.OIDC) error
}

type Live struct {
	store   Store
	mu      sync.Mutex
	current atomic.Pointer[db.OIDC]

	providerMu sync.Mutex
	cached     *provider
	failed     *failure
	fetching   *fetch
}

func NewLive(store Store, initial db.OIDC) *Live {
	l := &Live{store: store}
	initial = clone(initial)
	l.current.Store(&initial)

	return l
}

func (l *Live) Get() db.OIDC {
	return clone(*l.current.Load())
}

func (l *Live) Update(ctx context.Context, o db.OIDC) error {
	if err := Validate(o); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	o = clone(o)
	o.ExternalURL = externalOrigin(o.ExternalURL)

	l.mu.Lock()
	defer l.mu.Unlock()

	if err := l.store.UpdateOIDC(ctx, o); err != nil {
		return fmt.Errorf("save OIDC settings: %w", err)
	}

	if stored, err := l.store.GetOIDC(ctx); err == nil {
		o = clone(stored)
	}

	l.current.Store(&o)

	return nil
}

func clone(o db.OIDC) db.OIDC {
	o.Scopes = slices.Clone(o.Scopes)
	o.GroupRoles = slices.Clone(o.GroupRoles)

	return o
}

func RedirectURL(o db.OIDC) string {
	return o.ExternalURL + CallbackPath
}

func PostLogoutRedirectURL(o db.OIDC) string {
	return o.ExternalURL + PostLogoutPath
}

func Validate(o db.OIDC) error {
	errs := []error{
		validateSessionLifetime(o.SessionLifetime),
		validateScopes(o.Scopes),
		validateGroups(o.GroupsClaim, o.GroupRoles),
		ValidateCACertificate(o.CACertificate),
	}

	if o.Enabled || o.ExternalURL != "" {
		errs = append(errs, validateExternalURL(o.ExternalURL))
	}

	if o.Enabled || o.Issuer != "" {
		errs = append(errs, ValidateIssuer(o.Issuer))
	}

	errs = append(errs,
		validateText("client_id", o.ClientID, maxValue, o.Enabled),
		validateText("client_secret", o.ClientSecret, maxValue, o.Enabled),
		validateText("display_name", o.DisplayName, maxDisplayName, o.Enabled),
	)

	return errors.Join(errs...)
}

func validateSessionLifetime(d time.Duration) error {
	if d < MinSessionLifetime || d > MaxSessionLifetime {
		return fmt.Errorf("session_lifetime must be between %s and %s", MinSessionLifetime, MaxSessionLifetime)
	}

	return nil
}

func validateExternalURL(s string) error {
	u, err := parseHTTPS(s)
	if err != nil {
		return fmt.Errorf("external_url %w", err)
	}

	if u.Path != "" && u.Path != "/" || u.RawPath != "" {
		return errors.New("external_url must have no path")
	}

	return nil
}

func externalOrigin(s string) string {
	if validateExternalURL(s) != nil {
		return s
	}

	u, _ := url.Parse(s)

	return u.Scheme + "://" + strings.ToLower(u.Host)
}

func ValidateIssuer(s string) error {
	if _, err := parseHTTPS(s); err != nil {
		return fmt.Errorf("issuer %w", err)
	}

	return nil
}

func parseHTTPS(s string) (*url.URL, error) {
	if s == "" {
		return nil, errors.New("is required")
	}

	u, err := url.Parse(s)
	if err != nil || len(s) > maxURL || u.Scheme != "https" || u.Host == "" || u.Opaque != "" {
		return nil, errors.New("must be an https URL")
	}

	if u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(s, "#") {
		return nil, errors.New("must have no user information, query, or fragment")
	}

	return u, nil
}

func validateScopes(scopes []string) error {
	if !slices.Contains(scopes, "openid") || !slices.Contains(scopes, "email") {
		return errors.New("scopes must include openid and email")
	}

	if len(scopes) > MaxScopes {
		return fmt.Errorf("scopes must have at most %d values", MaxScopes)
	}

	for i, s := range scopes {
		if s == "" || len(s) > maxValue || strings.ContainsFunc(s, func(r rune) bool {
			return r < 0x21 || r == 0x22 || r == 0x5c || r > 0x7e
		}) {
			return fmt.Errorf("scope %q is not valid", s)
		}

		if slices.Contains(scopes[:i], s) {
			return fmt.Errorf("scope %q is repeated", s)
		}
	}

	return nil
}

func validateGroups(claim string, groupRoles []db.GroupRole) error {
	if err := validateText("groups_claim", claim, maxValue, len(groupRoles) > 0); err != nil {
		return err
	}

	if len(groupRoles) > MaxGroupRoles {
		return fmt.Errorf("group_roles must have at most %d values", MaxGroupRoles)
	}

	for i, g := range groupRoles {
		if err := validateText("group", g.Group, maxValue, true); err != nil {
			return err
		}

		if !slices.Contains([]db.RoleID{db.RoleAdmin, db.RoleReadOnly, db.RoleNetworkManager}, g.RoleID) {
			return fmt.Errorf("group %q has an invalid role", g.Group)
		}

		for _, prev := range groupRoles[:i] {
			if prev.Group == g.Group {
				return fmt.Errorf("group %q is repeated", g.Group)
			}
		}
	}

	return nil
}

func ValidateCACertificate(s string) error {
	if s == "" {
		return nil
	}

	if len(s) > maxCACertificate {
		return errors.New("ca_certificate is too long")
	}

	_, err := certPool(s)

	return err
}

func certPool(s string) (*x509.CertPool, error) {
	certs, err := CACertificates(s)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()

	for _, cert := range certs {
		pool.AddCert(cert)
	}

	return pool, nil
}

func CACertificates(s string) ([]*x509.Certificate, error) {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	rest := []byte(strings.Join(lines, "\n"))

	var certs []*x509.Certificate

	for {
		var block *pem.Block

		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			return nil, errors.New("ca_certificate must hold only PEM certificates")
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("ca_certificate: %w", err)
		}

		certs = append(certs, cert)
	}

	if strings.TrimSpace(string(rest)) != "" || len(certs) == 0 {
		return nil, errors.New("ca_certificate must hold one or more PEM certificates")
	}

	return certs, nil
}

func validateText(name, s string, maxLength int, required bool) error {
	switch {
	case s == "" && required:
		return fmt.Errorf("%s is required", name)
	case len(s) > maxLength:
		return fmt.Errorf("%s must be at most %d characters", name, maxLength)
	case strings.ContainsFunc(s, unicode.IsControl):
		return fmt.Errorf("%s must have no control characters", name)
	}

	return nil
}

func (l *Live) Reload(o db.OIDC) {
	o = clone(o)

	l.mu.Lock()
	defer l.mu.Unlock()

	l.current.Store(&o)
}
