// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"net/url"
)

type RoamerRegistration struct {
	RAT             string `json:"rat"`
	NodeID          string `json:"nodeID,omitempty"`
	Reachable       bool   `json:"reachable"`
	Registered      bool   `json:"registered"`
	ConnectionState string `json:"connectionState,omitempty"`
	Since           string `json:"since,omitempty"`
	LastSeenAt      string `json:"lastSeenAt,omitempty"`
	Radio           string `json:"radio,omitempty"`
}

type RoamerSession struct {
	RAT         string `json:"rat"`
	NodeID      string `json:"nodeID,omitempty"`
	ID          uint8  `json:"id"`
	DataNetwork string `json:"dataNetwork,omitempty"`
	IPv4Address string `json:"ipv4Address,omitempty"`
	IPv6Prefix  string `json:"ipv6Prefix,omitempty"`
}

type RoamerUsage struct {
	UplinkBytes   int64 `json:"uplinkBytes"`
	DownlinkBytes int64 `json:"downlinkBytes"`
	TotalBytes    int64 `json:"totalBytes"`
}

type Roamer struct {
	IMSI          string               `json:"imsi"`
	PartnerID     string               `json:"partnerID"`
	PartnerName   string               `json:"partnerName"`
	HomePLMN      RoamingPLMN          `json:"homePLMN"`
	Registrations []RoamerRegistration `json:"registrations"`
	Sessions      []RoamerSession      `json:"sessions"`
	UsageToday    *RoamerUsage         `json:"usageToday,omitempty"`
}

type UnreachableNode struct {
	NodeID string `json:"nodeID"`
	Error  string `json:"error"`
}

type ListRoamersResponse struct {
	Items            []Roamer          `json:"items"`
	UnreachableNodes []UnreachableNode `json:"unreachableNodes,omitempty"`
}

func (c *Client) ListRoamers(ctx context.Context) (*ListRoamersResponse, error) {
	var roamers ListRoamersResponse

	if err := c.roamingDo(ctx, "GET", "api/v1/roaming/roamers", nil, nil, &roamers); err != nil {
		return nil, err
	}

	return &roamers, nil
}

func (c *Client) GetRoamer(ctx context.Context, imsi string) (*Roamer, error) {
	var roamer Roamer

	if err := c.roamingDo(ctx, "GET", "api/v1/roaming/roamers/"+url.PathEscape(imsi), nil, nil, &roamer); err != nil {
		return nil, err
	}

	return &roamer, nil
}

func (c *Client) DeregisterRoamer(ctx context.Context, imsi string) error {
	return c.roamingDo(ctx, "DELETE", "api/v1/roaming/roamers/"+url.PathEscape(imsi), nil, nil, nil)
}
