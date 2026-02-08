package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type LoginOptions struct {
	Email    string
	Password string
}

type RefreshResponseResult struct {
	Token string `json:"token"`
}

// Login authenticates the user with the provided email and password.
// On success the server sets a session cookie. Call Refresh afterwards
// to obtain a JWT access token for subsequent authenticated requests.
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

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:    SyncRequest,
		Method:  "POST",
		Path:    "api/v1/auth/login",
		Body:    &body,
		Headers: headers,
	})
	if err != nil {
		return err
	}

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
