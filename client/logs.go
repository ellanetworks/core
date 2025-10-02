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

type GetSubscriberLogsRetentionPolicy struct {
	Days int `json:"days"`
}

type UpdateSubscriberLogsRetentionPolicyOptions struct {
	Days int `json:"days"`
}

type SubscriberLog struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	IMSI      string `json:"imsi"`
	Event     string `json:"event"`
	Details   string `json:"details"`
}

type ListSubscriberLogsResponse struct {
	Items      []SubscriberLog `json:"items"`
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
	TotalCount int             `json:"total_count"`
}

type GetRadioLogsRetentionPolicy struct {
	Days int `json:"days"`
}

type UpdateRadioLogsRetentionPolicyOptions struct {
	Days int `json:"days"`
}

type RadioLog struct {
	ID        int    `json:"id"`
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	RanID     string `json:"ran_id"`
	Event     string `json:"event"`
	Details   string `json:"details"`
}

type ListRadioLogsResponse struct {
	Items      []RadioLog `json:"items"`
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

func (c *Client) ListSubscriberLogs(ctx context.Context, p *ListParams) (*ListSubscriberLogsResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/logs/subscriber",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var subscriberLogs ListSubscriberLogsResponse

	err = resp.DecodeResult(&subscriberLogs)
	if err != nil {
		return nil, err
	}

	return &subscriberLogs, nil
}

func (c *Client) ClearSubscriberLogs(ctx context.Context) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/logs/subscriber",
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetSubscriberLogRetentionPolicy(ctx context.Context) (*GetSubscriberLogsRetentionPolicy, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/logs/subscriber/retention",
	})
	if err != nil {
		return nil, err
	}

	var policy GetSubscriberLogsRetentionPolicy

	err = resp.DecodeResult(&policy)
	if err != nil {
		return nil, err
	}

	return &policy, nil
}

func (c *Client) UpdateSubscriberLogRetentionPolicy(ctx context.Context, opts *UpdateSubscriberLogsRetentionPolicyOptions) error {
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
		Path:   "api/v1/logs/subscriber/retention",
		Body:   &body,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ListRadioLogs(ctx context.Context, p *ListParams) (*ListRadioLogsResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/logs/radio",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var radioLogs ListRadioLogsResponse

	err = resp.DecodeResult(&radioLogs)
	if err != nil {
		return nil, err
	}

	return &radioLogs, nil
}

func (c *Client) ClearRadioLogs(ctx context.Context) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/logs/radio",
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetRadioLogRetentionPolicy(ctx context.Context) (*GetRadioLogsRetentionPolicy, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/logs/radio/retention",
	})
	if err != nil {
		return nil, err
	}

	var policy GetRadioLogsRetentionPolicy

	err = resp.DecodeResult(&policy)
	if err != nil {
		return nil, err
	}

	return &policy, nil
}

func (c *Client) UpdateRadioLogRetentionPolicy(ctx context.Context, opts *UpdateRadioLogsRetentionPolicyOptions) error {
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
		Path:   "api/v1/logs/radio/retention",
		Body:   &body,
	})
	if err != nil {
		return err
	}
	return nil
}
