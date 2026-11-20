// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package oidc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"golang.org/x/oauth2"
)

const (
	httpTimeout     = 10 * time.Second
	MaxResponseSize = 1 << 20
	maxRedirects    = 10
)

const (
	ClientSecretBasic = "client_secret_basic"
	ClientSecretPost  = "client_secret_post"
)

var (
	ErrProvider      = errors.New("the identity provider")
	errTooLarge      = fmt.Errorf("the response is larger than %d bytes", MaxResponseSize)
	errInsecure      = errors.New("redirect to a URL that is not https")
	errNoRedirect    = errors.New("redirect refused")
	errTooManyRedirs = errors.New("too many redirects")
)

var signingAlgorithms = []string{
	gooidc.RS256, gooidc.RS384, gooidc.RS512, gooidc.ES256, gooidc.ES384, gooidc.ES512,
	gooidc.PS256, gooidc.PS384, gooidc.PS512, gooidc.EdDSA,
}

type Discovery struct {
	Issuer                  string
	AuthorizationEndpoint   string
	TokenEndpoint           string
	UserInfoEndpoint        string
	EndSessionEndpoint      string
	JWKSURI                 string
	SigningAlgorithms       []string
	TokenEndpointAuthMethod string
	Keys                    int
	PKCE                    bool
	IssuerParameter         bool
}

func (d Discovery) endpoint() oauth2.Endpoint {
	style := oauth2.AuthStyleInHeader
	if d.TokenEndpointAuthMethod == ClientSecretPost {
		style = oauth2.AuthStyleInParams
	}

	return oauth2.Endpoint{AuthURL: d.AuthorizationEndpoint, TokenURL: d.TokenEndpoint, AuthStyle: style}
}

type metadata struct {
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	UserInfoEndpoint      string   `json:"userinfo_endpoint"`
	EndSessionEndpoint    string   `json:"end_session_endpoint"`
	JWKSURI               string   `json:"jwks_uri"`
	ResponseTypes         []string `json:"response_types_supported"`
	SigningAlgorithms     []string `json:"id_token_signing_alg_values_supported"`
	TokenEndpointAuth     []string `json:"token_endpoint_auth_methods_supported"`
	CodeChallengeMethods  []string `json:"code_challenge_methods_supported"`
	IssuerParameter       bool     `json:"authorization_response_iss_parameter_supported"`
}

type limitedTransport struct {
	next http.RoundTripper
}

func (t limitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.next.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.ContentLength > MaxResponseSize {
		_ = resp.Body.Close()
		return nil, errTooLarge
	}

	resp.Body = &limitedBody{body: resp.Body, left: MaxResponseSize}

	return resp, nil
}

type limitedBody struct {
	body io.ReadCloser
	left int64
}

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.left <= 0 {
		var one [1]byte

		n, err := b.body.Read(one[:])
		if n > 0 {
			return 0, errTooLarge
		}

		return 0, err
	}

	if int64(len(p)) > b.left {
		p = p[:b.left]
	}

	n, err := b.body.Read(p)
	b.left -= int64(n)

	return n, err
}

func (b *limitedBody) Close() error {
	return b.body.Close()
}

func HTTPClient(caCertificate string) (*http.Client, error) {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("unexpected default HTTP transport")
	}

	transport = transport.Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}

	if caCertificate != "" {
		pool, err := certPool(caCertificate)
		if err != nil {
			return nil, err
		}

		transport.TLSClientConfig.RootCAs = pool
	}

	return &http.Client{Transport: limitedTransport{next: transport}, Timeout: httpTimeout, CheckRedirect: httpsOnly}, nil
}

func httpsOnly(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errTooManyRedirs
	}

	if req.URL.Scheme != "https" {
		return errInsecure
	}

	return nil
}

func withoutRedirects(c *http.Client) *http.Client {
	out := *c
	out.CheckRedirect = func(*http.Request, []*http.Request) error { return errNoRedirect }

	return &out
}

func Discover(ctx context.Context, issuer, caCertificate string) (Discovery, error) {
	if err := errors.Join(ValidateIssuer(issuer), ValidateCACertificate(caCertificate)); err != nil {
		return Discovery{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	client, err := HTTPClient(caCertificate)
	if err != nil {
		return Discovery{}, fmt.Errorf("%w: %w", ErrInvalid, err)
	}

	_, d, err := discover(ctx, client, issuer)
	if err != nil {
		return Discovery{}, err
	}

	if d.Keys, err = countSigningKeys(ctx, client, d.JWKSURI); err != nil {
		return Discovery{}, fmt.Errorf("%w: keys: %w", ErrProvider, err)
	}

	return d, nil
}

func discover(ctx context.Context, client *http.Client, issuer string) (*gooidc.Provider, Discovery, error) {
	provider, err := gooidc.NewProvider(gooidc.ClientContext(ctx, client), issuer)
	if err != nil {
		return nil, Discovery{}, fmt.Errorf("%w: discovery: %w", ErrProvider, err)
	}

	var m metadata
	if err := provider.Claims(&m); err != nil {
		return nil, Discovery{}, fmt.Errorf("%w: discovery: %w", ErrProvider, err)
	}

	d, err := m.check(issuer)
	if err != nil {
		return nil, Discovery{}, fmt.Errorf("%w: discovery: %w", ErrProvider, err)
	}

	return provider, d, nil
}

func (m metadata) check(issuer string) (Discovery, error) {
	endpoints := map[string]string{
		"authorization_endpoint": m.AuthorizationEndpoint,
		"token_endpoint":         m.TokenEndpoint,
		"jwks_uri":               m.JWKSURI,
	}

	if m.UserInfoEndpoint != "" {
		endpoints["userinfo_endpoint"] = m.UserInfoEndpoint
	}

	if m.EndSessionEndpoint != "" {
		endpoints["end_session_endpoint"] = m.EndSessionEndpoint
	}

	for name, endpoint := range endpoints {
		if u, err := url.Parse(endpoint); err != nil || u.Scheme != "https" || u.Host == "" {
			return Discovery{}, fmt.Errorf("%s must be an https URL", name)
		}
	}

	if !slices.Contains(m.ResponseTypes, "code") {
		return Discovery{}, errors.New("the provider does not support the authorization code flow")
	}

	advertised := m.SigningAlgorithms
	if len(advertised) == 0 {
		advertised = []string{gooidc.RS256}
	}

	var algorithms []string

	for _, alg := range advertised {
		if slices.Contains(signingAlgorithms, alg) {
			algorithms = append(algorithms, alg)
		}
	}

	if len(algorithms) == 0 {
		return Discovery{}, fmt.Errorf("the provider signs ID tokens with %v, and none is an asymmetric algorithm", advertised)
	}

	authMethod, err := tokenEndpointAuthMethod(m.TokenEndpointAuth)
	if err != nil {
		return Discovery{}, err
	}

	return Discovery{
		Issuer:                  issuer,
		AuthorizationEndpoint:   m.AuthorizationEndpoint,
		TokenEndpoint:           m.TokenEndpoint,
		UserInfoEndpoint:        m.UserInfoEndpoint,
		EndSessionEndpoint:      m.EndSessionEndpoint,
		JWKSURI:                 m.JWKSURI,
		SigningAlgorithms:       algorithms,
		TokenEndpointAuthMethod: authMethod,
		PKCE:                    slices.Contains(m.CodeChallengeMethods, "S256"),
		IssuerParameter:         m.IssuerParameter,
	}, nil
}

func tokenEndpointAuthMethod(supported []string) (string, error) {
	switch {
	case len(supported) == 0 || slices.Contains(supported, ClientSecretBasic):
		return ClientSecretBasic, nil
	case slices.Contains(supported, ClientSecretPost):
		return ClientSecretPost, nil
	default:
		return "", fmt.Errorf("the token endpoint supports %v, and neither %s nor %s", supported, ClientSecretBasic,
			ClientSecretPost)
	}
}

func countSigningKeys(ctx context.Context, client *http.Client, jwksURI string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return 0, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%s returned %s", jwksURI, resp.Status)
	}

	var set struct {
		Keys []json.RawMessage `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return 0, fmt.Errorf("%s: %w", jwksURI, err)
	}

	n := 0

	for _, raw := range set.Keys {
		var k jose.JSONWebKey
		if k.UnmarshalJSON(raw) != nil {
			continue
		}

		if (k.Use == "" || k.Use == "sig") && k.IsPublic() && k.Valid() {
			n++
		}
	}

	if n == 0 {
		return 0, fmt.Errorf("%s has no public signing key", jwksURI)
	}

	return n, nil
}
