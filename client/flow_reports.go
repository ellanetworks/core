// SPDX-FileCopyrightText: 2026-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type FlowReport struct {
	ID              int    `json:"id"`
	SubscriberID    string `json:"subscriber_id"`
	SourceIP        string `json:"source_ip"`
	DestinationIP   string `json:"destination_ip"`
	SourcePort      uint16 `json:"source_port"`
	DestinationPort uint16 `json:"destination_port"`
	Protocol        uint8  `json:"protocol"`
	Packets         uint64 `json:"packets"`
	Bytes           uint64 `json:"bytes"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
}

type ListFlowReportsResponse struct {
	Items      []FlowReport `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

type ListFlowReportsParams struct {
	Page          int    `json:"page"`
	PerPage       int    `json:"per_page"`
	SubscriberID  string `json:"subscriber_id"`
	Protocol      string `json:"protocol"`
	SourceIP      string `json:"source_ip"`
	DestinationIP string `json:"destination_ip"`
	Start         string `json:"start"`
	End           string `json:"end"`
}

type GroupedFlowReportsResponse []map[string][]FlowReport

type GetFlowReportsRetentionPolicy struct {
	Days int `json:"days"`
}

type UpdateFlowReportsRetentionPolicyOptions struct {
	Days int `json:"days"`
}

func buildFlowReportQuery(p *ListFlowReportsParams) url.Values {
	query := url.Values{}

	if p.Page != 0 {
		query.Set("page", fmt.Sprintf("%d", p.Page))
	}

	if p.PerPage != 0 {
		query.Set("per_page", fmt.Sprintf("%d", p.PerPage))
	}

	if p.SubscriberID != "" {
		query.Set("subscriber_id", p.SubscriberID)
	}

	if p.Protocol != "" {
		query.Set("protocol", p.Protocol)
	}

	if p.SourceIP != "" {
		query.Set("source_ip", p.SourceIP)
	}

	if p.DestinationIP != "" {
		query.Set("destination_ip", p.DestinationIP)
	}

	if p.Start != "" {
		query.Set("start", p.Start)
	}

	if p.End != "" {
		query.Set("end", p.End)
	}

	return query
}

// ListFlowReports retrieves a paginated list of flow reports with optional filtering.
func (c *Client) ListFlowReports(ctx context.Context, p *ListFlowReportsParams) (*ListFlowReportsResponse, error) {
	query := buildFlowReportQuery(p)

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/flow-reports",
		Query:  query,
	})
	if err != nil {
		return nil, err
	}

	var flowReports ListFlowReportsResponse

	err = resp.DecodeResult(&flowReports)
	if err != nil {
		return nil, err
	}

	return &flowReports, nil
}

// ListFlowReportsByDay retrieves flow reports grouped by day.
func (c *Client) ListFlowReportsByDay(ctx context.Context, p *ListFlowReportsParams) (*GroupedFlowReportsResponse, error) {
	query := buildFlowReportQuery(p)
	query.Set("group_by", "day")

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/flow-reports",
		Query:  query,
	})
	if err != nil {
		return nil, err
	}

	var grouped GroupedFlowReportsResponse

	err = resp.DecodeResult(&grouped)
	if err != nil {
		return nil, err
	}

	return &grouped, nil
}

// ListFlowReportsBySubscriber retrieves flow reports grouped by subscriber.
func (c *Client) ListFlowReportsBySubscriber(ctx context.Context, p *ListFlowReportsParams) (*GroupedFlowReportsResponse, error) {
	query := buildFlowReportQuery(p)
	query.Set("group_by", "subscriber")

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/flow-reports",
		Query:  query,
	})
	if err != nil {
		return nil, err
	}

	var grouped GroupedFlowReportsResponse

	err = resp.DecodeResult(&grouped)
	if err != nil {
		return nil, err
	}

	return &grouped, nil
}

// ClearFlowReports deletes all flow reports. Reports will be permanently deleted.
func (c *Client) ClearFlowReports(ctx context.Context) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/flow-reports",
	})
	if err != nil {
		return err
	}

	return nil
}

// GetFlowReportsRetentionPolicy retrieves the current flow reports retention policy.
func (c *Client) GetFlowReportsRetentionPolicy(ctx context.Context) (*GetFlowReportsRetentionPolicy, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/flow-reports/retention",
	})
	if err != nil {
		return nil, err
	}

	var policy GetFlowReportsRetentionPolicy

	err = resp.DecodeResult(&policy)
	if err != nil {
		return nil, err
	}

	return &policy, nil
}

// UpdateFlowReportsRetentionPolicy updates the flow reports retention policy.
func (c *Client) UpdateFlowReportsRetentionPolicy(ctx context.Context, opts *UpdateFlowReportsRetentionPolicyOptions) error {
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
		Path:   "api/v1/flow-reports/retention",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}
