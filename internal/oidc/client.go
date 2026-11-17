// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/ellanetworks/core/internal/db"
	"golang.org/x/oauth2"
)

const (
	providerTTL = time.Hour
	failureTTL  = 15 * time.Second
	maxIssuedAt = 5 * time.Minute
)

var (
	ErrDisabled = errors.New("OIDC is disabled")
	ErrIssuer   = errors.New("the authorization response comes from another issuer")
	ErrToken    = errors.New("invalid ID token")
	ErrUserInfo = errors.New("invalid UserInfo response")
	ErrOverage  = errors.New("the ID token refers to the groups instead of listing them")
)

type Verified int

const (
	VerifiedUnknown Verified = iota
	VerifiedTrue
	VerifiedFalse
)

type provider struct {
	key       string
	at        time.Time
	http      *http.Client
	provider  *gooidc.Provider
	discovery Discovery
}

type failure struct {
	key string
	at  time.Time
	err error
}

type fetch struct {
	key  string
	done chan struct{}
	p    *provider
	err  error
}

type Client struct {
	settings  db.OIDC
	http      *http.Client
	direct    *http.Client
	oauth     oauth2.Config
	provider  *gooidc.Provider
	verifier  *gooidc.IDTokenVerifier
	discovery Discovery
	now       func() time.Time
}

type Claims struct {
	Subject       string
	Email         string
	EmailVerified Verified
	Groups        []string
	GroupsOverage bool
}

func (l *Live) Client(ctx context.Context, now func() time.Time) (*Client, error) {
	o := l.Get()
	if !o.Enabled {
		return nil, ErrDisabled
	}

	p, err := l.provider(ctx, o, now())
	if err != nil {
		return nil, err
	}

	return &Client{
		settings: o,
		http:     p.http,
		direct:   withoutRedirects(p.http),
		oauth: oauth2.Config{
			ClientID:     o.ClientID,
			ClientSecret: o.ClientSecret,
			Endpoint:     p.discovery.endpoint(),
			RedirectURL:  RedirectURL(o),
			Scopes:       o.Scopes,
		},
		provider:  p.provider,
		verifier:  p.provider.Verifier(&gooidc.Config{ClientID: o.ClientID, Now: now}),
		discovery: p.discovery,
		now:       now,
	}, nil
}

func (l *Live) provider(ctx context.Context, o db.OIDC, now time.Time) (*provider, error) {
	key := o.Issuer + "\x00" + o.CACertificate

	l.providerMu.Lock()

	if p := l.cached; p != nil && p.key == key && now.Sub(p.at) < providerTTL && !now.Before(p.at) {
		l.providerMu.Unlock()
		return p, nil
	}

	if f := l.failed; f != nil && f.key == key && now.Sub(f.at) < failureTTL && !now.Before(f.at) {
		l.providerMu.Unlock()
		return nil, f.err
	}

	f := l.fetching
	if f == nil || f.key != key {
		f = &fetch{key: key, done: make(chan struct{})}
		l.fetching = f

		go l.fetch(context.WithoutCancel(ctx), f, o.Issuer, o.CACertificate, now)
	}

	l.providerMu.Unlock()

	select {
	case <-f.done:
		return f.p, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (l *Live) fetch(ctx context.Context, f *fetch, issuer, caCertificate string, now time.Time) {
	f.p, f.err = newProvider(ctx, f.key, issuer, caCertificate, now)

	l.providerMu.Lock()

	if f.err == nil {
		l.cached = f.p
	} else {
		l.failed = &failure{key: f.key, at: now, err: f.err}
	}

	if l.fetching == f {
		l.fetching = nil
	}

	l.providerMu.Unlock()
	close(f.done)
}

func newProvider(ctx context.Context, key, issuer, caCertificate string, now time.Time) (*provider, error) {
	client, err := HTTPClient(caCertificate)
	if err != nil {
		return nil, err
	}

	gp, d, err := discover(ctx, client, issuer)
	if err != nil {
		return nil, err
	}

	return &provider{key: key, at: now, http: client, provider: gp, discovery: d}, nil
}

func (c *Client) Settings() db.OIDC {
	return clone(c.settings)
}

func (c *Client) AuthCodeURL(state, nonce, verifier string) string {
	return c.oauth.AuthCodeURL(state, gooidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
}

func (c *Client) CheckIssuer(iss string, present bool) error {
	if present && iss != c.settings.Issuer || !present && c.discovery.IssuerParameter {
		return ErrIssuer
	}

	return nil
}

func (c *Client) Exchange(ctx context.Context, code, verifier, nonce string) (Claims, error) {
	token, err := c.oauth.Exchange(context.WithValue(ctx, oauth2.HTTPClient, c.direct), code,
		oauth2.VerifierOption(verifier))
	if err != nil {
		return Claims{}, fmt.Errorf("%w: token request: %w", ErrProvider, err)
	}

	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return Claims{}, fmt.Errorf("%w: the token response has no ID token", ErrToken)
	}

	fields, err := c.verify(ctx, raw, nonce)
	if err != nil {
		return Claims{}, err
	}

	cl, err := c.claims(fields, ErrToken)
	if err != nil || !c.incomplete(cl, fields) || c.discovery.UserInfoEndpoint == "" {
		return cl, err
	}

	info, err := c.userInfo(ctx, token, cl.Subject)
	if err != nil {
		return Claims{}, err
	}

	return c.merge(cl, fields, info)
}

func (c *Client) verify(ctx context.Context, raw, nonce string) (map[string]json.RawMessage, error) {
	token, err := c.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrToken, err)
	}

	var fields map[string]json.RawMessage
	if err := token.Claims(&fields); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrToken, err)
	}

	var azp string
	if err := optional(fields, "azp", &azp, ErrToken); err != nil {
		return nil, err
	}

	switch {
	case token.Nonce != nonce:
		return nil, fmt.Errorf("%w: wrong nonce", ErrToken)
	case len(token.Audience) != 1:
		return nil, fmt.Errorf("%w: audiences other than %q", ErrToken, c.settings.ClientID)
	case azp != "" && azp != c.settings.ClientID:
		return nil, fmt.Errorf("%w: issued to %q", ErrToken, azp)
	case token.IssuedAt.IsZero():
		return nil, fmt.Errorf("%w: no issue time", ErrToken)
	case token.IssuedAt.After(c.now().Add(maxIssuedAt)):
		return nil, fmt.Errorf("%w: issued in the future", ErrToken)
	case token.Subject == "" || len(token.Subject) > maxValue:
		return nil, fmt.Errorf("%w: the subject must be 1 to %d characters", ErrToken, maxValue)
	}

	return fields, nil
}

func (c *Client) userInfo(ctx context.Context, token *oauth2.Token, subject string) (map[string]json.RawMessage, error) {
	if !strings.EqualFold(token.TokenType, "Bearer") {
		return nil, fmt.Errorf("%w: the access token is of type %q, not Bearer", ErrUserInfo, token.TokenType)
	}

	info, err := c.provider.UserInfo(gooidc.ClientContext(ctx, c.direct),
		oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token.AccessToken, TokenType: "Bearer"}))
	if err != nil {
		return nil, fmt.Errorf("%w: UserInfo request: %w", ErrProvider, err)
	}

	var fields map[string]json.RawMessage
	if err := info.Claims(&fields); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUserInfo, err)
	}

	var sub string
	if err := optional(fields, "sub", &sub, ErrUserInfo); err != nil {
		return nil, err
	}

	if sub != subject {
		return nil, fmt.Errorf("%w: the subject is not the ID token's", ErrUserInfo)
	}

	return fields, nil
}

func (c *Client) incomplete(cl Claims, fields map[string]json.RawMessage) bool {
	return cl.Email == "" || cl.EmailVerified == VerifiedUnknown || c.MapsGroups() && !hasGroups(fields, c.settings.GroupsClaim)
}

func (c *Client) merge(cl Claims, idToken, userInfo map[string]json.RawMessage) (Claims, error) {
	info, err := c.claims(userInfo, ErrUserInfo)
	if err != nil {
		return Claims{}, err
	}

	switch {
	case cl.Email == "":
		cl.Email, cl.EmailVerified = info.Email, info.EmailVerified
	case cl.EmailVerified == VerifiedUnknown && info.Email == cl.Email:
		cl.EmailVerified = info.EmailVerified
	}

	if c.MapsGroups() && !hasGroups(idToken, c.settings.GroupsClaim) {
		cl.Groups, cl.GroupsOverage = info.Groups, info.GroupsOverage
	}

	return cl, nil
}

func (c *Client) claims(fields map[string]json.RawMessage, kind error) (Claims, error) {
	var cl Claims

	if err := optional(fields, "sub", &cl.Subject, kind); err != nil {
		return Claims{}, err
	}

	if err := optional(fields, "email", &cl.Email, kind); err != nil {
		return Claims{}, err
	}

	var verified any
	if err := optional(fields, "email_verified", &verified, kind); err != nil {
		return Claims{}, err
	}

	switch verified {
	case nil:
		cl.EmailVerified = VerifiedUnknown
	case true, "true":
		cl.EmailVerified = VerifiedTrue
	default:
		cl.EmailVerified = VerifiedFalse
	}

	if c.settings.GroupsClaim == "" {
		return cl, nil
	}

	cl.GroupsOverage = overage(fields, c.settings.GroupsClaim)

	raw, ok := fields[c.settings.GroupsClaim]
	if !ok || string(raw) == "null" {
		return cl, nil
	}

	var one string
	if json.Unmarshal(raw, &one) == nil {
		cl.Groups = []string{one}
		return cl, nil
	}

	if err := json.Unmarshal(raw, &cl.Groups); err != nil {
		return Claims{}, fmt.Errorf("%w: the %s claim is not a list of strings", kind, c.settings.GroupsClaim)
	}

	return cl, nil
}

func hasGroups(fields map[string]json.RawMessage, claim string) bool {
	raw, ok := fields[claim]

	return ok && string(raw) != "null" || overage(fields, claim)
}

func overage(fields map[string]json.RawMessage, claim string) bool {
	var names map[string]json.RawMessage
	if json.Unmarshal(fields["_claim_names"], &names) != nil {
		return false
	}

	_, ok := names[claim]

	return ok
}

func optional(fields map[string]json.RawMessage, name string, v any, kind error) error {
	raw, ok := fields[name]
	if !ok || string(raw) == "null" {
		return nil
	}

	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("%w: the %s claim: %w", kind, name, err)
	}

	return nil
}

func (c *Client) RoleFor(groups []string) (db.RoleID, bool) {
	for _, g := range c.settings.GroupRoles {
		if slices.Contains(groups, g.Group) {
			return g.RoleID, true
		}
	}

	return 0, false
}

func (c *Client) MapsGroups() bool {
	return len(c.settings.GroupRoles) > 0
}

func (c *Client) TrustsEmail(cl Claims) bool {
	switch {
	case strings.TrimSpace(cl.Email) == "":
		return false
	case cl.EmailVerified == VerifiedTrue:
		return true
	case cl.EmailVerified == VerifiedUnknown:
		return c.settings.TrustUnverifiedEmail
	default:
		return false
	}
}
