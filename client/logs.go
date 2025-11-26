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
