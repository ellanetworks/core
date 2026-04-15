package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

type ClusterMember struct {
	NodeID        int    `json:"nodeId"`
	RaftAddress   string `json:"raftAddress"`
	APIAddress    string `json:"apiAddress"`
	BinaryVersion string `json:"binaryVersion"`
	Suffrage      string `json:"suffrage"`
}

type DrainOptions struct {
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
}

type DrainResponse struct {
	Message               string `json:"message"`
	TransferredLeadership bool   `json:"transferredLeadership"`
	RANsNotified          int    `json:"ransNotified"`
	BGPStopped            bool   `json:"bgpStopped"`
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

func (c *Client) DrainNode(ctx context.Context, opts *DrainOptions) (*DrainResponse, error) {
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
		Path:   "api/v1/cluster/drain",
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

func (c *Client) RemoveClusterMember(ctx context.Context, nodeID int) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   fmt.Sprintf("api/v1/cluster/members/%d", nodeID),
	})
	if err != nil {
		return err
	}

	return nil
}
