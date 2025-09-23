package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

type RoleID int

const (
	RoleAdmin          RoleID = 1
	RoleReadOnly       RoleID = 2
	RoleNetworkManager RoleID = 3
)

type CreateUserOptions struct {
	Email    string `json:"email"`
	RoleID   RoleID `json:"role_id"`
	Password string `json:"password"`
}

type DeleteUserOptions struct {
	Email string `json:"email"`
}

type CreateAPITokenOptions struct {
	Name   string `json:"name"`
	Expiry string `json:"expiry,omitempty"` // ISO 8601 format, optional
}

type CreateAPITokenResponse struct {
	Token string `json:"token"`
}

type APIToken struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Expiry string `json:"expiry,omitempty"` // ISO 8601 format, optional
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
		Path:   fmt.Sprintf("api/v1/users?page=%d&per_page=%d", p.Page, p.PerPage),
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

func (c *Client) CreateMyAPIToken(ctx context.Context, opts *CreateAPITokenOptions) (*CreateAPITokenResponse, error) {
	payload := struct {
		Name   string `json:"name"`
		Expiry string `json:"expiry,omitempty"` // ISO 8601 format, optional
	}{
		Name:   opts.Name,
		Expiry: opts.Expiry,
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

func (c *Client) ListMyAPITokens(ctx context.Context, p *ListParams) (*ListAPITokensResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   fmt.Sprintf("api/v1/users/me/api-tokens?page=%d&per_page=%d", p.Page, p.PerPage),
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
