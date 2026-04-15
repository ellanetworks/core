package client

import "context"

type ClusterStatus struct {
	Enabled      bool   `json:"enabled"`
	Role         string `json:"role"`
	NodeID       int    `json:"nodeId"`
	AppliedIndex uint64 `json:"appliedIndex"`
}

type Status struct {
	Version     string         `json:"version"`
	Initialized bool           `json:"initialized"`
	Ready       bool           `json:"ready"`
	Cluster     *ClusterStatus `json:"cluster,omitempty"`
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
