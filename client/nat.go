package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type GetNATInfoResponse struct {
	Enabled bool `json:"enabled,omitempty"`
}

type UpdateNATInfoOptions struct {
	Enabled bool `json:"enabled"`
}

// GetNATInfo retrieves the current Network Address Translation (NAT) configuration.
func (c *Client) GetNATInfo(ctx context.Context) (*GetNATInfoResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/nat",
	})
	if err != nil {
		return nil, err
	}

	var natInfoResponse GetNATInfoResponse

	err = resp.DecodeResult(&natInfoResponse)
	if err != nil {
		return nil, err
	}

	return &natInfoResponse, nil
}

// UpdateNATInfo updates the Network Address Translation (NAT) configuration.
func (c *Client) UpdateNATInfo(ctx context.Context, opts *UpdateNATInfoOptions) error {
	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(opts)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/networking/nat",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}
