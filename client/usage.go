package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
)

type GetUsageRetentionPolicy struct {
	Days int `json:"days"`
}

type UpdateUsageRetentionPolicyOptions struct {
	Days int `json:"days"`
}

type SubscriberUsage struct {
	UplinkBytes   int64 `json:"uplink_bytes"`
	DownlinkBytes int64 `json:"downlink_bytes"`
	TotalBytes    int64 `json:"total_bytes"`
}

type ListUsageResponse []map[string]SubscriberUsage

type ListUsageParams struct {
	Start      string `json:"start"`
	End        string `json:"end"`
	GroupBy    string `json:"group_by"`
	Subscriber string `json:"subscriber"`
}

func (c *Client) ListUsage(ctx context.Context, p *ListUsageParams) (*ListUsageResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/subscriber-usage",
		Query: url.Values{
			"start":      {p.Start},
			"end":        {p.End},
			"group_by":   {p.GroupBy},
			"subscriber": {p.Subscriber},
		},
	})
	if err != nil {
		return nil, err
	}

	var usage ListUsageResponse

	err = resp.DecodeResult(&usage)
	if err != nil {
		return nil, err
	}

	return &usage, nil
}

func (c *Client) GetUsageRetentionPolicy(ctx context.Context) (*GetUsageRetentionPolicy, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/subscriber-usage/retention",
	})
	if err != nil {
		return nil, err
	}

	var policy GetUsageRetentionPolicy

	err = resp.DecodeResult(&policy)
	if err != nil {
		return nil, err
	}

	return &policy, nil
}

func (c *Client) UpdateUsageRetentionPolicy(ctx context.Context, opts *UpdateUsageRetentionPolicyOptions) error {
	payload := struct {
		Days int `json:"days"`
	}{
		Days: opts.Days,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/subscriber-usage/retention",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}
