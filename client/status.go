package client

import "context"

// PendingMigration describes the cluster's schema-migration readiness.
// Non-nil only during a rolling-upgrade window: the local binary
// supports a higher schema than the cluster has currently applied.
type PendingMigration struct {
	// CurrentSchema is the schema version the cluster has applied.
	CurrentSchema int `json:"currentSchema"`

	// TargetSchema is the highest version the cluster could migrate
	// to right now, bounded by the local binary's max and every
	// voter's MaxSchemaVersion. Equals CurrentSchema when blocked
	// behind a laggard voter.
	TargetSchema int `json:"targetSchema"`

	// LaggardNodeId is the voter holding the migration up; non-zero
	// only when target == current (blocked).
	LaggardNodeId int `json:"laggardNodeId,omitempty"`
}

type ClusterStatus struct {
	Enabled          bool   `json:"enabled"`
	Role             string `json:"role"`
	NodeID           int    `json:"nodeId"`
	IsLeader         bool   `json:"isLeader"`
	LeaderNodeID     int    `json:"leaderNodeId"`
	AppliedIndex     uint64 `json:"appliedIndex"`
	ClusterID        string `json:"clusterId,omitempty"`
	LeaderAPIAddress string `json:"leaderAPIAddress,omitempty"`

	// AppliedSchemaVersion is the schema version every node in the
	// cluster has committed. Differs from the parent SchemaVersion
	// (which is the local binary's max) during a rolling upgrade.
	AppliedSchemaVersion int `json:"appliedSchemaVersion"`

	// PendingMigration is non-nil during a rolling upgrade when the
	// local binary supports schema beyond what's applied. See
	// PendingMigration.
	PendingMigration *PendingMigration `json:"pendingMigration,omitempty"`
}

type Status struct {
	Version       string         `json:"version"`
	Revision      string         `json:"revision,omitempty"`
	Initialized   bool           `json:"initialized"`
	Ready         bool           `json:"ready"`
	SchemaVersion int            `json:"schemaVersion"`
	Cluster       *ClusterStatus `json:"cluster,omitempty"`
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
