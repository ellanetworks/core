// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

type LoginOptions struct {
	Email    string
	Password string
}

type LoginResponseResult struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}

type RefreshResponseResult struct {
	Token string `json:"token"`
}

// Login authenticates the user with the provided email and password.
// On success, it stores the returned access token in the client.
func (c *Client) Login(ctx context.Context, opts *LoginOptions) error {
	payload := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{
		Email:    opts.Email,
		Password: opts.Password,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:    SyncRequest,
		Method:  "POST",
		Path:    "api/v1/auth/login",
		Body:    &body,
		Headers: headers,
	})
	if err != nil {
		return err
	}

	var loginResponse LoginResponseResult

	err = resp.DecodeResult(&loginResponse)
	if err != nil {
		return err
	}

	c.token = loginResponse.Token

	return nil
}

// Refresh refreshes the authentication token.
// It stores the new token in the client.
func (c *Client) Refresh(ctx context.Context) error {
	headers := map[string]string{
		"Content-Type": "application/json",
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:    SyncRequest,
		Method:  "POST",
		Path:    "api/v1/auth/refresh",
		Body:    nil,
		Headers: headers,
	})
	if err != nil {
		return err
	}

	var refreshResponse RefreshResponseResult

	err = resp.DecodeResult(&refreshResponse)
	if err != nil {
		return err
	}

	c.token = refreshResponse.Token

	return nil
}

type LookupTokenResult struct {
	Valid bool `json:"valid"`
}

// LookupToken reports whether the credentials the client is configured with are
// accepted by the server.
func (c *Client) LookupToken(ctx context.Context) (*LookupTokenResult, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/auth/lookup-token",
	})
	if err != nil {
		return nil, err
	}

	var result LookupTokenResult

	if err := resp.DecodeResult(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// RotateSecret rotates the JWT signing secret, invalidating existing login
// sessions. API-token authentication is unaffected.
func (c *Client) RotateSecret(ctx context.Context) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/auth/rotate-secret",
	})
	if err != nil {
		return err
	}

	return nil
}

// Logout clears the caller's session cookie. It is a no-op for API-token
// authentication. The endpoint replies with 204 and an empty body.
func (c *Client) Logout(ctx context.Context) error {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   RawRequest,
		Method: "POST",
		Path:   "api/v1/auth/logout",
	})
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("logout: unexpected status %d", resp.StatusCode)
	}

	return nil
}
