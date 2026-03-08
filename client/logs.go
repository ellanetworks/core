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

// ListAuditLogsParams contains all filter and pagination options for listing audit logs.
type ListAuditLogsParams struct {
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	Actor   string `json:"actor"`
	Start   string `json:"start"`
	End     string `json:"end"`
}

func buildAuditLogQuery(p *ListAuditLogsParams) url.Values {
	query := url.Values{}

	if p.Page > 0 {
		query.Set("page", fmt.Sprintf("%d", p.Page))
	}

	if p.PerPage > 0 {
		query.Set("per_page", fmt.Sprintf("%d", p.PerPage))
	}

	if p.Actor != "" {
		query.Set("actor", p.Actor)
	}

	if p.Start != "" {
		query.Set("start", p.Start)
	}

	if p.End != "" {
		query.Set("end", p.End)
	}

	return query
}

// ListAuditLogs retrieves a paginated list of audit logs with optional filters.
func (c *Client) ListAuditLogs(ctx context.Context, p *ListAuditLogsParams) (*ListAuditLogsResponse, error) {
	query := buildAuditLogQuery(p)

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/logs/audit",
		Query:  query,
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

// ListAuditLogsByActor retrieves a paginated list of audit logs filtered by actor email.
// Deprecated: Use ListAuditLogs with ListAuditLogsParams.Actor instead.
func (c *Client) ListAuditLogsByActor(ctx context.Context, actor string, p *ListParams) (*ListAuditLogsResponse, error) {
	return c.ListAuditLogs(ctx, &ListAuditLogsParams{
		Page:    p.Page,
		PerPage: p.PerPage,
		Actor:   actor,
	})
}

// GetAuditLogRetentionPolicy retrieves the current audit log retention policy.
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

// UpdateAuditLogRetentionPolicy updates the audit log retention policy.
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
