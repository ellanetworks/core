package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type InitializeOptions struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type InitializeResponseResult struct {
	Message string `json:"message"`
	Token   string `json:"token"`
}

// Initialize initializes the client with the provided options.
// On success, it stores the returned access token in the client.
func (c *Client) Initialize(ctx context.Context, opts *InitializeOptions) error {
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

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/init",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	var initResponse InitializeResponseResult

	err = resp.DecodeResult(&initResponse)
	if err != nil {
		return err
	}

	c.token = initResponse.Token

	return nil
}
