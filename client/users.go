package client

import (
	"bytes"
	"context"
	"encoding/json"
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

type User struct {
	Email  string `json:"email"`
	RoleID RoleID `json:"role_id"`
}

func (c *Client) ListUsers() ([]*User, error) {
	resp, err := c.Requester.Do(context.Background(), &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/users",
	})
	if err != nil {
		return nil, err
	}
	var users []*User
	err = resp.DecodeResult(&users)
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (c *Client) CreateUser(opts *CreateUserOptions) error {
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

	_, err = c.Requester.Do(context.Background(), &RequestOptions{
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

func (c *Client) DeleteUser(opts *DeleteUserOptions) error {
	_, err := c.Requester.Do(context.Background(), &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/users/" + opts.Email,
	})
	if err != nil {
		return err
	}
	return nil
}
