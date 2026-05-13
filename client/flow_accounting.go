package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type GetFlowAccountingInfoResponse struct {
	Enabled bool `json:"enabled"`
}

type UpdateFlowAccountingInfoOptions struct {
	Enabled bool `json:"enabled"`
}

// GetFlowAccountingInfo retrieves the current flow accounting configuration.
func (c *Client) GetFlowAccountingInfo(ctx context.Context) (*GetFlowAccountingInfoResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/flow-accounting",
	})
	if err != nil {
		return nil, err
	}

	var flowAccountingResponse GetFlowAccountingInfoResponse

	err = resp.DecodeResult(&flowAccountingResponse)
	if err != nil {
		return nil, err
	}

	return &flowAccountingResponse, nil
}

// UpdateFlowAccountingInfo updates the flow accounting configuration.
func (c *Client) UpdateFlowAccountingInfo(ctx context.Context, opts *UpdateFlowAccountingInfoOptions) error {
	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(opts)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/networking/flow-accounting",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}
