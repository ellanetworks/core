package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

type ClusterMember struct {
	NodeID           int    `json:"nodeId"`
	RaftAddress      string `json:"raftAddress"`
	APIAddress       string `json:"apiAddress"`
	BinaryVersion    string `json:"binaryVersion"`
	Suffrage         string `json:"suffrage"`
	MaxSchemaVersion int    `json:"maxSchemaVersion"`
	IsLeader         bool   `json:"isLeader"`
	DrainState       string `json:"drainState"`
	DrainUpdatedAt   string `json:"drainUpdatedAt,omitempty"`
}

type DrainOptions struct {
	DeadlineSeconds int `json:"deadlineSeconds,omitempty"`
}

type DrainResponse struct {
	Message               string `json:"message"`
	State                 string `json:"state"`
	TransferredLeadership bool   `json:"transferredLeadership"`
	RANsNotified          int    `json:"ransNotified"`
	BGPStopped            bool   `json:"bgpStopped"`
	SessionsRemaining     int    `json:"sessionsRemaining"`
}

type ResumeResponse struct {
	Message    string `json:"message"`
	State      string `json:"state"`
	BGPStarted bool   `json:"bgpStarted"`
}

// AutopilotServer is the live per-peer health reported by raft-autopilot.
type AutopilotServer struct {
	NodeID          int    `json:"nodeId"`
	RaftAddress     string `json:"raftAddress"`
	NodeStatus      string `json:"nodeStatus"`
	Healthy         bool   `json:"healthy"`
	IsLeader        bool   `json:"isLeader"`
	HasVotingRights bool   `json:"hasVotingRights"`
	LastContactMs   int64  `json:"lastContactMs"`
	LastTerm        uint64 `json:"lastTerm"`
	LastIndex       uint64 `json:"lastIndex"`
	StableSince     string `json:"stableSince,omitempty"`
}

// AutopilotState is the cluster-wide live health snapshot. Autopilot
// runs leader-only; followers proxy the request transparently.
type AutopilotState struct {
	Healthy          bool              `json:"healthy"`
	FailureTolerance int               `json:"failureTolerance"`
	LeaderNodeID     int               `json:"leaderNodeId"`
	Voters           []int             `json:"voters"`
	Servers          []AutopilotServer `json:"servers"`
}

func (c *Client) ListClusterMembers(ctx context.Context) ([]ClusterMember, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/cluster/members",
	})
	if err != nil {
		return nil, err
	}

	var members []ClusterMember

	err = resp.DecodeResult(&members)
	if err != nil {
		return nil, err
	}

	return members, nil
}

func (c *Client) GetClusterMember(ctx context.Context, nodeID int) (*ClusterMember, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   fmt.Sprintf("api/v1/cluster/members/%d", nodeID),
	})
	if err != nil {
		return nil, err
	}

	var member ClusterMember

	err = resp.DecodeResult(&member)
	if err != nil {
		return nil, err
	}

	return &member, nil
}

// GetAutopilotState returns the live autopilot view. Safe to call from
// any node — the server proxies to the leader when needed.
func (c *Client) GetAutopilotState(ctx context.Context) (*AutopilotState, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/cluster/autopilot",
	})
	if err != nil {
		return nil, err
	}

	var state AutopilotState

	err = resp.DecodeResult(&state)
	if err != nil {
		return nil, err
	}

	return &state, nil
}

// DrainClusterMember drains the given node. When opts.DeadlineSeconds > 0 the
// server returns as soon as the drain starts (state "draining") and finalises
// asynchronously when the node's last active lease clears or the deadline
// elapses. When opts.DeadlineSeconds == 0 (default), the drain is synchronous
// and the response state is "drained".
func (c *Client) DrainClusterMember(ctx context.Context, nodeID int, opts *DrainOptions) (*DrainResponse, error) {
	var body bytes.Buffer

	if opts != nil {
		err := json.NewEncoder(&body).Encode(opts)
		if err != nil {
			return nil, err
		}
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   fmt.Sprintf("api/v1/cluster/members/%d/drain", nodeID),
		Body:   &body,
	})
	if err != nil {
		return nil, err
	}

	var drainResp DrainResponse

	err = resp.DecodeResult(&drainResp)
	if err != nil {
		return nil, err
	}

	return &drainResp, nil
}

// ResumeClusterMember reverses a prior drain: restarts the BGP speaker
// (if BGP is enabled) and clears drain state. AMF Status Indication and
// transferred Raft leadership are not reversed.
func (c *Client) ResumeClusterMember(ctx context.Context, nodeID int) (*ResumeResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   fmt.Sprintf("api/v1/cluster/members/%d/resume", nodeID),
	})
	if err != nil {
		return nil, err
	}

	var out ResumeResponse

	err = resp.DecodeResult(&out)
	if err != nil {
		return nil, err
	}

	return &out, nil
}

// PromoteClusterMember promotes a non-voter to a voter immediately.
// Autopilot also promotes stable non-voters automatically after a short
// stabilization window; use this call when you need promotion without
// waiting.
func (c *Client) PromoteClusterMember(ctx context.Context, nodeID int) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   fmt.Sprintf("api/v1/cluster/members/%d/promote", nodeID),
	})
	if err != nil {
		return err
	}

	return nil
}

// RemoveClusterMember removes a node. The server requires the node's
// drainState to be "drained" first; pass force=true to skip that check
// (use sparingly — skipping drain leaves the node's dynamic IP leases and
// RAN connections uncleaned until the removal itself triggers the usual
// post-remove purge).
func (c *Client) RemoveClusterMember(ctx context.Context, nodeID int, force bool) error {
	path := fmt.Sprintf("api/v1/cluster/members/%d", nodeID)
	if force {
		path += "?force=true"
	}

	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   path,
	})
	if err != nil {
		return err
	}

	return nil
}
