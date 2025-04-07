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

func (c *Client) Login(opts *LoginOptions) (*LoginResponseResult, error) {
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
		return nil, err
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	var loginResponse LoginResponseResult

	resp, err := c.Requester.Do(context.Background(), &RequestOptions{
		Type:    SyncRequest,
		Method:  "POST",
		Path:    "api/v1/auth/login",
		Body:    &body,
		Headers: headers,
	})
	if err != nil {
		return nil, err
	}

	err = resp.DecodeResult(&loginResponse)
	if err != nil {
		return nil, err
	}

	return &loginResponse, nil
}
