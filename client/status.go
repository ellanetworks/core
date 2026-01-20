package client

import "context"

type Status struct {
	Version     string `json:"version"`
	Initialized bool   `json:"initialized"`
}

// GetStatus retrieves the current status of the system.
func (c *Client) GetStatus(ctx context.Context) (*Status, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/status",
	})
	if err != nil {
		return nil, err
	}

	var statusResponse Status

	err = resp.DecodeResult(&statusResponse)
	if err != nil {
		return nil, err
	}

	return &statusResponse, nil
}
