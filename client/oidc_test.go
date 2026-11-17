// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestGetOIDC_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"enabled": true, "issuer": "https://idp.example.com", "client_secret_set": true, "scopes": ["openid", "email"], "group_roles": [{"group": "noc", "role_id": 3}]}`),
		},
	}
	clientObj := &client.Client{Requester: fake}

	settings, err := clientObj.GetOIDC(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !settings.Enabled || settings.Issuer != "https://idp.example.com" || !settings.ClientSecretSet ||
		len(settings.GroupRoles) != 1 || settings.GroupRoles[0].RoleID != client.RoleNetworkManager {
		t.Fatalf("unexpected settings: %+v", settings)
	}

	if fake.lastOpts.Method != "GET" || fake.lastOpts.Path != "api/v1/oidc" {
		t.Fatalf("unexpected request: %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}

func TestUpdateOIDC_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"enabled": true, "redirect_url": "https://core.example.com/api/v1/auth/oidc/callback"}`),
		},
	}
	clientObj := &client.Client{Requester: fake}

	settings, err := clientObj.UpdateOIDC(context.Background(), &client.UpdateOIDCOptions{
		Enabled:                true,
		ExternalURL:            "https://core.example.com",
		Issuer:                 "https://idp.example.com",
		ClientID:               "ella-core",
		ClientSecret:           "secret",
		DisplayName:            "Keycloak",
		Scopes:                 []string{"openid", "email", "profile"},
		SessionLifetimeSeconds: 86400,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if settings.RedirectURL != "https://core.example.com/api/v1/auth/oidc/callback" {
		t.Fatalf("unexpected settings: %+v", settings)
	}

	if fake.lastOpts.Method != "PUT" || fake.lastOpts.Path != "api/v1/oidc" {
		t.Fatalf("unexpected request: %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	raw, err := io.ReadAll(fake.lastOpts.Body)
	if err != nil {
		t.Fatal(err)
	}

	var sent map[string]any
	if err := json.Unmarshal(raw, &sent); err != nil {
		t.Fatal(err)
	}

	if sent["client_secret"] != "secret" || sent["group_roles"] == nil {
		t.Fatalf("unexpected body: %s", raw)
	}
}

func TestTestOIDCProvider_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"issuer": "https://idp.example.com", "keys": 2, "pkce": true}`),
		},
	}
	clientObj := &client.Client{Requester: fake}

	provider, err := clientObj.TestOIDCProvider(context.Background(), &client.TestOIDCProviderOptions{Issuer: "https://idp.example.com"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if provider.Keys != 2 || !provider.PKCE || fake.lastOpts.Path != "api/v1/oidc/discovery" {
		t.Fatalf("unexpected provider %+v or path %s", provider, fake.lastOpts.Path)
	}
}

func TestRemoveUserPasswordAndUnlinkUser(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{StatusCode: 200, Headers: http.Header{}, Result: []byte(`{"message": "ok"}`)},
	}
	clientObj := &client.Client{Requester: fake}

	if err := clientObj.RemoveUserPassword(context.Background(), "a b@example.com"); err != nil {
		t.Fatalf("RemoveUserPassword: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" || fake.lastOpts.Path != "api/v1/users/a%20b@example.com/password" {
		t.Fatalf("unexpected request: %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}

	if err := clientObj.UnlinkUser(context.Background(), "user@example.com"); err != nil {
		t.Fatalf("UnlinkUser: %v", err)
	}

	if fake.lastOpts.Method != "DELETE" || fake.lastOpts.Path != "api/v1/users/user@example.com/oidc" {
		t.Fatalf("unexpected request: %s %s", fake.lastOpts.Method, fake.lastOpts.Path)
	}
}
