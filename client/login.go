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

type LoginResponseResult struct {
	Token string `json:"token"`
}

// Login authenticates the user with the provided email and password.
// It stores the token in the client for future requests.
func (c *Client) Login(opts *LoginOptions) error {
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

	resp, err := c.Requester.Do(context.Background(), &RequestOptions{
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
