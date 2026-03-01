package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type GetFleetURLResponse struct {
	URL string `json:"url"`
}

type UpdateFleetURLOptions struct {
	URL string `json:"url"`
}

type RegisterFleetOptions struct {
	ActivationToken string `json:"activationToken"`
}

// GetFleetURL retrieves the configured Fleet server URL.
func (c *Client) GetFleetURL(ctx context.Context) (*GetFleetURLResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/fleet/url",
	})
	if err != nil {
		return nil, err
	}

	var response GetFleetURLResponse

	err = resp.DecodeResult(&response)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

// UpdateFleetURL sets the Fleet server URL.
func (c *Client) UpdateFleetURL(ctx context.Context, opts *UpdateFleetURLOptions) error {
	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(opts)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/fleet/url",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// RegisterFleet registers the Core instance with a Fleet using an activation token.
func (c *Client) RegisterFleet(ctx context.Context, opts *RegisterFleetOptions) error {
	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(opts)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/fleet/register",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// UnregisterFleet unregisters the Core instance from a Fleet.
func (c *Client) UnregisterFleet(ctx context.Context) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/fleet/unregister",
	})
	if err != nil {
		return err
	}

	return nil
}
