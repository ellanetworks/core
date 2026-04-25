package client

import "context"

type ClusterStatus struct {
	Enabled          bool   `json:"enabled"`
	Role             string `json:"role"`
	NodeID           int    `json:"nodeId"`
	IsLeader         bool   `json:"isLeader"`
	LeaderNodeID     int    `json:"leaderNodeId"`
	AppliedIndex     uint64 `json:"appliedIndex"`
	ClusterID        string `json:"clusterId,omitempty"`
	LeaderAPIAddress string `json:"leaderAPIAddress,omitempty"`
}

type FleetStatus struct {
	Managed    bool   `json:"managed"`
	LastSyncAt string `json:"lastSyncAt,omitempty"`
}

type Status struct {
	Version       string         `json:"version"`
	Revision      string         `json:"revision,omitempty"`
	Initialized   bool           `json:"initialized"`
	Ready         bool           `json:"ready"`
	SchemaVersion int            `json:"schemaVersion"`
	Cluster       *ClusterStatus `json:"cluster,omitempty"`
	Fleet         FleetStatus    `json:"fleet"`
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
