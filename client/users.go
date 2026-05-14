package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type RoleID int

const (
	RoleAdmin          RoleID = 1
	RoleReadOnly       RoleID = 2
	RoleNetworkManager RoleID = 3
)

type UpdateMyPasswordOptions struct {
	CurrentPassword string `json:"current_password"`
	Password        string `json:"password"`
}

type UpdateUserPasswordOptions struct {
	Password string `json:"password"`
}

type CreateUserOptions struct {
	Email    string `json:"email"`
	RoleID   RoleID `json:"role_id"`
	Password string `json:"password"`
}

type UpdateUserOptions struct {
	RoleID RoleID `json:"role_id"`
}

type GetUserOptions struct {
	Email string `json:"email"`
}

type DeleteUserOptions struct {
	Email string `json:"email"`
}

type CreateAPITokenOptions struct {
	Name string `json:"name"`
	// ExpiresAt is an optional RFC 3339 timestamp. Empty means "no expiry".
	ExpiresAt string `json:"expires_at,omitempty"`
}

type CreateAPITokenResponse struct {
	Token string `json:"token"`
}

type APIToken struct {
	ID   string `json:"id"` // public token identifier (token_id), not the DB primary key
	Name string `json:"name"`
	// ExpiresAt is an RFC 3339 timestamp; empty when the token has no expiry.
	ExpiresAt string `json:"expires_at,omitempty"`
}

type User struct {
	Email  string `json:"email"`
	RoleID RoleID `json:"role_id"`
}

type ListUsersResponse struct {
	Items      []User `json:"items"`
	Page       int    `json:"page"`
	PerPage    int    `json:"per_page"`
	TotalCount int    `json:"total_count"`
}

type ListAPITokensResponse struct {
	Items      []APIToken `json:"items"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
	TotalCount int        `json:"total_count"`
}

func (c *Client) ListUsers(ctx context.Context, p *ListParams) (*ListUsersResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/users",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var users ListUsersResponse

	err = resp.DecodeResult(&users)
	if err != nil {
		return nil, err
	}

	return &users, nil
}

func (c *Client) CreateUser(ctx context.Context, opts *CreateUserOptions) error {
	payload := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		RoleID   RoleID `json:"role_id"`
	}{
		Email:    opts.Email,
		Password: opts.Password,
		RoleID:   opts.RoleID,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/users",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetUser(ctx context.Context, opts *GetUserOptions) (*User, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/users/" + opts.Email,
	})
	if err != nil {
		return nil, err
	}

	var user User

	err = resp.DecodeResult(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdateUser changes a user's role. Password changes go through
// UpdateUserPassword.
func (c *Client) UpdateUser(ctx context.Context, email string, opts *UpdateUserOptions) error {
	payload := struct {
		RoleID RoleID `json:"role_id"`
	}{
		RoleID: opts.RoleID,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/users/" + email,
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) DeleteUser(ctx context.Context, opts *DeleteUserOptions) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/users/" + opts.Email,
	})
	if err != nil {
		return err
	}

	return nil
}

// CreateMyAPIToken creates an API token for the authenticated user.
func (c *Client) CreateMyAPIToken(ctx context.Context, opts *CreateAPITokenOptions) (*CreateAPITokenResponse, error) {
	payload := struct {
		Name      string `json:"name"`
		ExpiresAt string `json:"expires_at,omitempty"`
	}{
		Name:      opts.Name,
		ExpiresAt: opts.ExpiresAt,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/users/me/api-tokens",
		Body:   &body,
	})
	if err != nil {
		return nil, err
	}

	var tokenResponse CreateAPITokenResponse

	err = resp.DecodeResult(&tokenResponse)
	if err != nil {
		return nil, err
	}

	return &tokenResponse, nil
}

// ListMyAPITokens lists API tokens for the authenticated user.
func (c *Client) ListMyAPITokens(ctx context.Context, p *ListParams) (*ListAPITokensResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/users/me/api-tokens",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var tokens ListAPITokensResponse

	err = resp.DecodeResult(&tokens)
	if err != nil {
		return nil, err
	}

	return &tokens, nil
}

// DeleteMyAPIToken deletes an API token owned by the authenticated user.
func (c *Client) DeleteMyAPIToken(ctx context.Context, tokenID string) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/users/me/api-tokens/" + tokenID,
	})
	if err != nil {
		return err
	}

	return nil
}

// ListUserAPITokens lists API tokens for the given user. Admin-only.
func (c *Client) ListUserAPITokens(ctx context.Context, email string, p *ListParams) (*ListAPITokensResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/users/" + email + "/api-tokens",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var tokens ListAPITokensResponse

	err = resp.DecodeResult(&tokens)
	if err != nil {
		return nil, err
	}

	return &tokens, nil
}

// CreateUserAPIToken creates an API token for the given user. Admin-only.
func (c *Client) CreateUserAPIToken(ctx context.Context, email string, opts *CreateAPITokenOptions) (*CreateAPITokenResponse, error) {
	payload := struct {
		Name      string `json:"name"`
		ExpiresAt string `json:"expires_at,omitempty"`
	}{
		Name:      opts.Name,
		ExpiresAt: opts.ExpiresAt,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/users/" + email + "/api-tokens",
		Body:   &body,
	})
	if err != nil {
		return nil, err
	}

	var tokenResponse CreateAPITokenResponse

	err = resp.DecodeResult(&tokenResponse)
	if err != nil {
		return nil, err
	}

	return &tokenResponse, nil
}

// DeleteUserAPIToken deletes an API token from the given user. Admin-only.
func (c *Client) DeleteUserAPIToken(ctx context.Context, email string, tokenID string) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/users/" + email + "/api-tokens/" + tokenID,
	})
	if err != nil {
		return err
	}

	return nil
}

// UpdateMyPassword changes the password of the currently authenticated user.
func (c *Client) UpdateMyPassword(ctx context.Context, opts *UpdateMyPasswordOptions) error {
	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(opts)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/users/me/password",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// UpdateUserPassword changes the password of the specified user. Requires admin privileges.
func (c *Client) UpdateUserPassword(ctx context.Context, email string, opts *UpdateUserPasswordOptions) error {
	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(opts)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/users/" + email + "/password",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}
