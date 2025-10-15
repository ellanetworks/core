package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type AuditLog struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Actor     string `json:"actor"`
	Action    string `json:"action"`
	IP        string `json:"ip"`
	Details   string `json:"details"`
}

type GetAuditLogsRetentionPolicy struct {
	Days int `json:"days"`
}

type UpdateAuditLogsRetentionPolicyOptions struct {
	Days int `json:"days"`
}

type ListAuditLogsResponse struct {
	Items      []AuditLog `json:"items"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
	TotalCount int        `json:"total_count"`
}

type GetNetworkLogsRetentionPolicy struct {
	Days int `json:"days"`
}

type UpdateNetworkLogsRetentionPolicyOptions struct {
	Days int `json:"days"`
}

type NetworkLog struct {
	ID          int    `json:"id"`
	Timestamp   string `json:"timestamp"`
	Level       string `json:"level"`
	Protocol    string `json:"protocol"`
	MessageType string `json:"message_type"`
	Direction   string `json:"direction"`
	Raw         string `json:"raw"`
	Details     string `json:"details"`
}

type ListNetworkLogsResponse struct {
	Items      []NetworkLog `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

type ListNetworkLogsParams struct {
	Page          int    `json:"page"`
	PerPage       int    `json:"per_page"`
	Protocol      string `json:"protocol"`
	Direction     string `json:"direction"`
	MessageType   string `json:"message_type"`
	TimestampFrom string `json:"timestamp_from"`
	TimestampTo   string `json:"timestamp_to"`
}

type NetworkLogContent struct {
	Decoded any    `json:"decoded"`
	Raw     string `json:"raw"`
}

func (c *Client) ListAuditLogs(ctx context.Context, p *ListParams) (*ListAuditLogsResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/logs/audit",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var auditLogs ListAuditLogsResponse

	err = resp.DecodeResult(&auditLogs)
	if err != nil {
		return nil, err
	}

	return &auditLogs, nil
}

func (c *Client) GetAuditLogRetentionPolicy(ctx context.Context) (*GetAuditLogsRetentionPolicy, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/logs/audit/retention",
	})
	if err != nil {
		return nil, err
	}

	var policy GetAuditLogsRetentionPolicy

	err = resp.DecodeResult(&policy)
	if err != nil {
		return nil, err
	}

	return &policy, nil
}

func (c *Client) UpdateAuditLogRetentionPolicy(ctx context.Context, opts *UpdateAuditLogsRetentionPolicyOptions) error {
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
		Path:   "api/v1/logs/audit/retention",
		Body:   &body,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ListNetworkLogs(ctx context.Context, p *ListNetworkLogsParams) (*ListNetworkLogsResponse, error) {
	query := url.Values{}
	if p.Page != 0 {
		query.Set("page", fmt.Sprintf("%d", p.Page))
	}

	if p.PerPage != 0 {
		query.Set("per_page", fmt.Sprintf("%d", p.PerPage))
	}

	if p.Protocol != "" {
		query.Set("protocol", p.Protocol)
	}

	if p.Direction != "" {
		query.Set("direction", p.Direction)
	}

	if p.MessageType != "" {
		query.Set("message_type", p.MessageType)
	}

	if p.TimestampFrom != "" {
		query.Set("timestamp_from", p.TimestampFrom)
	}

	if p.TimestampTo != "" {
		query.Set("timestamp_to", p.TimestampTo)
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/logs/network",
		Query:  query,
	})
	if err != nil {
		return nil, err
	}

	var networkLogs ListNetworkLogsResponse

	err = resp.DecodeResult(&networkLogs)
	if err != nil {
		return nil, err
	}

	return &networkLogs, nil
}

func (c *Client) ClearNetworkLogs(ctx context.Context) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/logs/network",
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetNetworkLogRetentionPolicy(ctx context.Context) (*GetNetworkLogsRetentionPolicy, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/logs/network/retention",
	})
	if err != nil {
		return nil, err
	}

	var policy GetNetworkLogsRetentionPolicy

	err = resp.DecodeResult(&policy)
	if err != nil {
		return nil, err
	}

	return &policy, nil
}

func (c *Client) UpdateNetworkLogRetentionPolicy(ctx context.Context, opts *UpdateNetworkLogsRetentionPolicyOptions) error {
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
		Path:   "api/v1/logs/network/retention",
		Body:   &body,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetNetworkLog(ctx context.Context, id int) (*NetworkLogContent, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   fmt.Sprintf("api/v1/logs/network/%d", id),
	})
	if err != nil {
		return nil, err
	}

	var logContent NetworkLogContent

	err = resp.DecodeResult(&logContent)
	if err != nil {
		return nil, err
	}

	return &logContent, nil
}
