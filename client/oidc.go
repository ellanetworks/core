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
	PostLogoutRedirectURL  string          `json:"post_logout_redirect_url"`
	Issuer                 string          `json:"issuer"`
	ClientID               string          `json:"client_id"`
	ClientSecretSet        bool            `json:"client_secret_set"`
	DisplayName            string          `json:"display_name"`
	Scopes                 []string        `json:"scopes"`
	SessionLifetimeSeconds int64           `json:"session_lifetime_seconds"`
	TrustUnverifiedEmail   bool            `json:"trust_unverified_email"`
	CACertificate          string          `json:"ca_certificate"`
	CACertificates         []OIDCCACert    `json:"ca_certificates"`
	GroupsClaim            string          `json:"groups_claim"`
	GroupRoles             []OIDCGroupRole `json:"group_roles"`
}

type OIDCCACert struct {
	Subject  string `json:"subject"`
	NotAfter string `json:"not_after"`
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
