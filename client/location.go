// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

type MeasurementQuantities struct {
	NR    []string `json:"nr,omitempty"`
	EUTRA []string `json:"eutra,omitempty"`
}

type GetLocationOptions struct {
	SUPI                  string                 `json:"supi"`
	Method                string                 `json:"method,omitempty"`
	Mode                  string                 `json:"mode,omitempty"`
	MeasurementQuantities *MeasurementQuantities `json:"measurement_quantities,omitempty"`
}

type PeriodicEventInfo struct {
	ReportingAmount      int64  `json:"reporting_amount,omitempty"`
	ReportingInterval    int64  `json:"reporting_interval"`
	ReportingInfiniteInd bool   `json:"reporting_infinite_ind,omitempty"`
	ReportingIntervalMs  *int64 `json:"reporting_interval_ms,omitempty"`
}

type CreateDeferredLocationRequestOptions struct {
	SUPI                  string                 `json:"supi"`
	LdrType               string                 `json:"ldr_type"`
	PeriodicEventInfo     PeriodicEventInfo      `json:"periodic_event_info"`
	ScheduledLocTime      string                 `json:"scheduled_loc_time,omitempty"`
	Method                string                 `json:"method,omitempty"`
	Mode                  string                 `json:"mode,omitempty"`
	MeasurementQuantities *MeasurementQuantities `json:"measurement_quantities,omitempty"`
}

type DeferredLocationRequest struct {
	ID                    string                 `json:"id"`
	SUPI                  string                 `json:"supi"`
	LdrType               string                 `json:"ldr_type"`
	PeriodicEventInfo     PeriodicEventInfo      `json:"periodic_event_info"`
	ScheduledLocTime      string                 `json:"scheduled_loc_time,omitempty"`
	Method                string                 `json:"method"`
	Mode                  string                 `json:"mode,omitempty"`
	MeasurementQuantities *MeasurementQuantities `json:"measurement_quantities,omitempty"`
	Status                string                 `json:"status"`
	TerminationCause      string                 `json:"termination_cause,omitempty"`
	TerminationReason     string                 `json:"termination_reason,omitempty"`
	EventReportCounter    int64                  `json:"event_report_counter"`
	CreatedAt             string                 `json:"created_at"`
	TerminatedAt          string                 `json:"terminated_at,omitempty"`
}

type ListDeferredLocationRequestsParams struct {
	Page    int
	PerPage int
	SUPI    string
	Status  string
}

type ListDeferredLocationRequestsResponse struct {
	Items      []DeferredLocationRequest `json:"items"`
	Page       int                       `json:"page"`
	PerPage    int                       `json:"per_page"`
	TotalCount int                       `json:"total_count"`
}

type LocationReport struct {
	Seq          int64           `json:"seq"`
	ReportedAt   string          `json:"reported_at"`
	LocationData json.RawMessage `json:"location_data"`
}

type ListLocationReportsResponse struct {
	Items []LocationReport `json:"items"`
	Next  int64            `json:"next"`
}

func (c *Client) GetLocation(ctx context.Context, opts *GetLocationOptions) (json.RawMessage, error) {
	body, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:    SyncRequest,
		Method:  "POST",
		Path:    "api/beta/location",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    bytes.NewReader(body),
	})
	if err != nil {
		return nil, err
	}

	var data json.RawMessage

	if err := resp.DecodeResult(&data); err != nil {
		return nil, err
	}

	return data, nil
}

func (c *Client) CreateDeferredLocationRequest(ctx context.Context, opts *CreateDeferredLocationRequestOptions) (*DeferredLocationRequest, error) {
	body, err := json.Marshal(opts)
	if err != nil {
		return nil, err
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:    SyncRequest,
		Method:  "POST",
		Path:    "api/beta/location/deferred-requests",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    bytes.NewReader(body),
	})
	if err != nil {
		return nil, err
	}

	var request DeferredLocationRequest

	if err := resp.DecodeResult(&request); err != nil {
		return nil, err
	}

	return &request, nil
}

func (c *Client) ListDeferredLocationRequests(ctx context.Context, p *ListDeferredLocationRequestsParams) (*ListDeferredLocationRequestsResponse, error) {
	query := url.Values{}

	if p != nil {
		if p.Page > 0 {
			query.Set("page", strconv.Itoa(p.Page))
		}

		if p.PerPage > 0 {
			query.Set("per_page", strconv.Itoa(p.PerPage))
		}

		if p.SUPI != "" {
			query.Set("supi", p.SUPI)
		}

		if p.Status != "" {
			query.Set("status", p.Status)
		}
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/beta/location/deferred-requests",
		Query:  query,
	})
	if err != nil {
		return nil, err
	}

	var list ListDeferredLocationRequestsResponse

	if err := resp.DecodeResult(&list); err != nil {
		return nil, err
	}

	return &list, nil
}

func (c *Client) GetDeferredLocationRequest(ctx context.Context, id string) (*DeferredLocationRequest, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/beta/location/deferred-requests/" + url.PathEscape(id),
	})
	if err != nil {
		return nil, err
	}

	var request DeferredLocationRequest

	if err := resp.DecodeResult(&request); err != nil {
		return nil, err
	}

	return &request, nil
}

func (c *Client) DeleteDeferredLocationRequest(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("id is required")
	}

	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/beta/location/deferred-requests/" + url.PathEscape(id),
	})

	return err
}

func (c *Client) ListLocationReports(ctx context.Context, id string, after int64, limit int) (*ListLocationReportsResponse, error) {
	if id == "" {
		return nil, fmt.Errorf("id is required")
	}

	query := url.Values{"after": {strconv.FormatInt(after, 10)}}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/beta/location/deferred-requests/" + url.PathEscape(id) + "/reports",
		Query:  query,
	})
	if err != nil {
		return nil, err
	}

	var reports ListLocationReportsResponse

	if err := resp.DecodeResult(&reports); err != nil {
		return nil, err
	}

	return &reports, nil
}
