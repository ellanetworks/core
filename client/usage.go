// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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

type DailySubscriberUsage struct {
	Date          string `json:"date"`
	UplinkBytes   int64  `json:"uplink_bytes"`
	DownlinkBytes int64  `json:"downlink_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
}

type PerSubscriberUsage struct {
	IMSI          string `json:"imsi"`
	UplinkBytes   int64  `json:"uplink_bytes"`
	DownlinkBytes int64  `json:"downlink_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
}

type ListUsageParams struct {
	Start      string `json:"start"`
	End        string `json:"end"`
	Subscriber string `json:"subscriber"`
}

// ListUsagePerDay retrieves subscriber usage totalled per UTC day, oldest first.
func (c *Client) ListUsagePerDay(ctx context.Context, p *ListUsageParams) ([]DailySubscriberUsage, error) {
	return listUsage[DailySubscriberUsage](ctx, c, p, "day")
}

// ListUsagePerSubscriber retrieves subscriber usage totalled per IMSI, busiest first.
func (c *Client) ListUsagePerSubscriber(ctx context.Context, p *ListUsageParams) ([]PerSubscriberUsage, error) {
	return listUsage[PerSubscriberUsage](ctx, c, p, "subscriber")
}

func listUsage[T any](ctx context.Context, c *Client, p *ListUsageParams, groupBy string) ([]T, error) {
	query := url.Values{"group_by": {groupBy}}

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

	var usage []T

	if err := resp.DecodeResult(&usage); err != nil {
		return nil, err
	}

	return usage, nil
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
