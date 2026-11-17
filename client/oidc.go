// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type OIDCGroupRole struct {
	Group  string `json:"group"`
	RoleID RoleID `json:"role_id"`
}

type OIDCSettings struct {
	Enabled                bool            `json:"enabled"`
	ExternalURL            string          `json:"external_url"`
	RedirectURL            string          `json:"redirect_url"`
	Issuer                 string          `json:"issuer"`
	ClientID               string          `json:"client_id"`
	ClientSecretSet        bool            `json:"client_secret_set"`
	DisplayName            string          `json:"display_name"`
	Scopes                 []string        `json:"scopes"`
	SessionLifetimeSeconds int64           `json:"session_lifetime_seconds"`
	TrustUnverifiedEmail   bool            `json:"trust_unverified_email"`
	CACertificate          string          `json:"ca_certificate"`
	GroupsClaim            string          `json:"groups_claim"`
	GroupRoles             []OIDCGroupRole `json:"group_roles"`
}

type UpdateOIDCOptions struct {
	Enabled                bool            `json:"enabled"`
	ExternalURL            string          `json:"external_url,omitempty"`
	Issuer                 string          `json:"issuer,omitempty"`
	ClientID               string          `json:"client_id,omitempty"`
	ClientSecret           string          `json:"client_secret,omitempty"`
	DisplayName            string          `json:"display_name,omitempty"`
	Scopes                 []string        `json:"scopes"`
	SessionLifetimeSeconds int64           `json:"session_lifetime_seconds"`
	TrustUnverifiedEmail   bool            `json:"trust_unverified_email"`
	CACertificate          string          `json:"ca_certificate,omitempty"`
	GroupsClaim            string          `json:"groups_claim,omitempty"`
	GroupRoles             []OIDCGroupRole `json:"group_roles"`
}

type TestOIDCProviderOptions struct {
	Issuer        string `json:"issuer"`
	CACertificate string `json:"ca_certificate,omitempty"`
}

type OIDCProvider struct {
	Issuer                  string   `json:"issuer"`
	AuthorizationEndpoint   string   `json:"authorization_endpoint"`
	TokenEndpoint           string   `json:"token_endpoint"`
	UserInfoEndpoint        string   `json:"userinfo_endpoint"`
	JWKSURI                 string   `json:"jwks_uri"`
	SigningAlgorithms       []string `json:"signing_algorithms"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Keys                    int      `json:"keys"`
	PKCE                    bool     `json:"pkce"`
	IssuerParameter         bool     `json:"issuer_parameter"`
}

func (c *Client) GetOIDC(ctx context.Context) (*OIDCSettings, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/oidc",
	})
	if err != nil {
		return nil, err
	}

	var settings OIDCSettings

	if err := resp.DecodeResult(&settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func (c *Client) UpdateOIDC(ctx context.Context, opts *UpdateOIDCOptions) (*OIDCSettings, error) {
	payload := *opts
	if payload.GroupRoles == nil {
		payload.GroupRoles = []OIDCGroupRole{}
	}

	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return nil, err
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/oidc",
		Body:   &body,
	})
	if err != nil {
		return nil, err
	}

	var settings OIDCSettings

	if err := resp.DecodeResult(&settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func (c *Client) TestOIDCProvider(ctx context.Context, opts *TestOIDCProviderOptions) (*OIDCProvider, error) {
	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(opts); err != nil {
		return nil, err
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/oidc/discovery",
		Body:   &body,
	})
	if err != nil {
		return nil, err
	}

	var provider OIDCProvider

	if err := resp.DecodeResult(&provider); err != nil {
		return nil, err
	}

	return &provider, nil
}
