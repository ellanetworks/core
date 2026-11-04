// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

const withdrawalTimeout = 90 * time.Second

// Roamer is the summary form returned by ListRoamers.
type Roamer struct {
	Imsi     string           `json:"imsi"`
	Partner  string           `json:"partner"`
	HomePLMN *RoamingPLMN     `json:"home_plmn,omitempty"`
	Msisdn   string           `json:"msisdn,omitempty"`
	Since    string           `json:"since"`
	Status   SubscriberStatus `json:"status"`
}

type ListRoamersParams struct {
	Page        int    `json:"page"`
	PerPage     int    `json:"per_page"`
	Partner     string `json:"partner,omitempty"`
	Search      string `json:"search,omitempty"`
	Radio       string `json:"radio,omitempty"`
	DataNetwork string `json:"data_network,omitempty"`
}

type ListRoamersResponse struct {
	Items      []Roamer `json:"items"`
	Page       int      `json:"page"`
	PerPage    int      `json:"per_page"`
	TotalCount int      `json:"total_count"`
}

// RoamerDetail is the full form returned by GetRoamer.
type RoamerDetail struct {
	Imsi          string         `json:"imsi"`
	Partner       string         `json:"partner"`
	HomePLMN      *RoamingPLMN   `json:"home_plmn,omitempty"`
	Msisdn        string         `json:"msisdn,omitempty"`
	Since         string         `json:"since"`
	Registrations []Registration `json:"registrations"`
	Sessions      []Session      `json:"sessions"`
}

func (c *Client) ListRoamers(ctx context.Context, p *ListRoamersParams) (*ListRoamersResponse, error) {
	query := url.Values{
		"page":     {fmt.Sprintf("%d", p.Page)},
		"per_page": {fmt.Sprintf("%d", p.PerPage)},
	}

	if p.Partner != "" {
		query.Set("partner", p.Partner)
	}

	if p.Search != "" {
		query.Set("search", p.Search)
	}

	if p.Radio != "" {
		query.Set("radio", p.Radio)
	}

	if p.DataNetwork != "" {
		query.Set("data_network", p.DataNetwork)
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/roamers",
		Query:  query,
	})
	if err != nil {
		return nil, err
	}

	var roamers ListRoamersResponse

	if err := resp.DecodeResult(&roamers); err != nil {
		return nil, err
	}

	return &roamers, nil
}

func (c *Client) GetRoamer(ctx context.Context, imsi string) (*RoamerDetail, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/roamers/" + imsi,
	})
	if err != nil {
		return nil, err
	}

	var roamer RoamerDetail

	if err := resp.DecodeResult(&roamer); err != nil {
		return nil, err
	}

	return &roamer, nil
}

// DeregisterRoamer deregisters a roamer and answers once it is gone, within a minute.
func (c *Client) DeregisterRoamer(ctx context.Context, imsi string) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:    SyncRequest,
		Method:  "DELETE",
		Path:    "api/v1/roamers/" + imsi,
		Timeout: withdrawalTimeout,
	})

	return err
}
