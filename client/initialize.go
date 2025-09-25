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

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/init",
		Body:   &body,
	})
	if err != nil {
		return err
	}
	return nil
}
