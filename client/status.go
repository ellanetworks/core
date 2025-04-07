package client

import (
	"context"
)

type Status struct {
	Version     string `json:"version"`
	Initialized bool   `json:"initialized"`
}

func (c *Client) GetStatus() (*Status, error) {
	resp, err := c.Requester.Do(context.Background(), &RequestOptions{
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
