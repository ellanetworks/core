// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// ListUsage retrieves subscriber usage data based on the provided parameters.
// The server groups results and rejects any GroupBy other than "day" or
// "subscriber".
func (c *Client) ListUsage(ctx context.Context, p *ListUsageParams) (*ListUsageResponse, error) {
	if p.GroupBy != "day" && p.GroupBy != "subscriber" {
		return nil, fmt.Errorf("group_by must be \"day\" or \"subscriber\", got %q", p.GroupBy)
	}

	query := url.Values{"group_by": {p.GroupBy}}

	if p.Start != "" {
		query.Set("start", p.Start)
	}

	if p.End != "" {
		query.Set("end", p.End)
	}

	if p.Subscriber != "" {
		query.Set("subscriber", p.Subscriber)
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/subscriber-usage",
		Query:  query,
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

// ClearUsage deletes all recorded subscriber usage.
func (c *Client) ClearUsage(ctx context.Context) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/subscriber-usage",
	})
	if err != nil {
		return err
	}

	return nil
}

// GetUsageRetentionPolicy retrieves the current usage retention policy.
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

// UpdateUsageRetentionPolicy updates the usage retention policy with the provided options.
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
