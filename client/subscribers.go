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

type CreateSubscriberOptions struct {
	Imsi           string `json:"imsi"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequenceNumber"`
	ProfileName    string `json:"profile_name"`
	OPc            string `json:"opc,omitempty"`
}

type UpdateSubscriberOptions struct {
	ProfileName string `json:"profile_name"`
}

type GetSubscriberOptions struct {
	ID string `json:"id"`
}

type DeleteSubscriberOptions struct {
	ID string `json:"id"`
}

// SubscriberStatus is the lightweight status carried in list responses.
type SubscriberStatus struct {
	Registered      bool   `json:"registered"`
	RadioAccessType string `json:"radio_access_type,omitempty"`
	NumSessions     int    `json:"num_sessions"`
	LastSeenAt      string `json:"last_seen_at,omitempty"`
}

// Subscriber is the summary form returned by ListSubscribers.
type Subscriber struct {
	Imsi        string           `json:"imsi"`
	ProfileName string           `json:"profile_name"`
	Radio       string           `json:"radio,omitempty"`
	Status      SubscriberStatus `json:"status"`
}

type ListSubscribersParams struct {
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	Radio   string `json:"radio,omitempty"`
}

type ListSubscribersResponse struct {
	Items      []Subscriber `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

// SubscriberDetailStatus is the rich status carried in GetSubscriber responses.
type SubscriberDetailStatus struct {
	Registered         bool   `json:"registered"`
	RadioAccessType    string `json:"radio_access_type,omitempty"`
	Imei               string `json:"imei"`
	CipheringAlgorithm string `json:"ciphering_algorithm"`
	IntegrityAlgorithm string `json:"integrity_algorithm"`
	LastSeenAt         string `json:"last_seen_at,omitempty"`
	LastSeenRadio      string `json:"last_seen_radio,omitempty"`
}

// SubscriberDetail is the full form returned by GetSubscriber.
type SubscriberDetail struct {
	Imsi        string                 `json:"imsi"`
	ProfileName string                 `json:"profile_name"`
	Status      SubscriberDetailStatus `json:"status"`
	Sessions    []Session              `json:"sessions"`
}

// SessionSlice is the 5G network slice identifier (S-NSSAI) of a session;
// absent for 4G.
type SessionSlice struct {
	SST int32  `json:"sst"`
	SD  string `json:"sd,omitempty"`
}

// Session is a UE data session — a 5G PDU session or a 4G PDN connection —
// self-describing via RadioAccessType.
type Session struct {
	RadioAccessType string        `json:"radio_access_type"` // "4G" | "5G"
	ID              uint8         `json:"id"`                // PDU Session ID (5G) / linked EPS Bearer ID (4G)
	Status          string        `json:"status"`
	IPType          string        `json:"ip_type,omitempty"` // IPv4 | IPv6 | IPv4v6
	IPv4Address     string        `json:"ipv4_address,omitempty"`
	IPv6Prefix      string        `json:"ipv6_prefix,omitempty"`
	DataNetwork     string        `json:"data_network,omitempty"` // DNN (5G) / APN (4G)
	Slice           *SessionSlice `json:"slice,omitempty"`        // 5G only
	AMBRUplink      string        `json:"ambr_uplink,omitempty"`
	AMBRDownlink    string        `json:"ambr_downlink,omitempty"`
}

type SubscriberCredentials struct {
	Key            string `json:"key"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
}

type GetSubscriberCredentialsOptions struct {
	ID string `json:"id"`
}

func (c *Client) CreateSubscriber(ctx context.Context, opts *CreateSubscriberOptions) error {
	payload := struct {
		Imsi           string `json:"imsi"`
		Key            string `json:"key"`
		SequenceNumber string `json:"sequenceNumber"`
		ProfileName    string `json:"profile_name"`
		OPc            string `json:"opc,omitempty"`
	}{
		Imsi:           opts.Imsi,
		Key:            opts.Key,
		SequenceNumber: opts.SequenceNumber,
		ProfileName:    opts.ProfileName,
		OPc:            opts.OPc,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/subscribers",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetSubscriber(ctx context.Context, opts *GetSubscriberOptions) (*SubscriberDetail, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/subscribers/" + opts.ID,
	})
	if err != nil {
		return nil, err
	}

	var subscriberResponse SubscriberDetail

	err = resp.DecodeResult(&subscriberResponse)
	if err != nil {
		return nil, err
	}

	return &subscriberResponse, nil
}

// UpdateSubscriber moves a subscriber to a different profile.
// profile_name is the only settable field.
func (c *Client) UpdateSubscriber(ctx context.Context, imsi string, opts *UpdateSubscriberOptions) error {
	payload := struct {
		ProfileName string `json:"profile_name"`
	}{
		ProfileName: opts.ProfileName,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/subscribers/" + imsi,
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) DeleteSubscriber(ctx context.Context, opts *DeleteSubscriberOptions) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/subscribers/" + opts.ID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) ListSubscribers(ctx context.Context, p *ListSubscribersParams) (*ListSubscribersResponse, error) {
	query := url.Values{
		"page":     {fmt.Sprintf("%d", p.Page)},
		"per_page": {fmt.Sprintf("%d", p.PerPage)},
	}

	if p.Radio != "" {
		query.Set("radio", p.Radio)
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/subscribers",
		Query:  query,
	})
	if err != nil {
		return nil, err
	}

	var subscribers ListSubscribersResponse

	err = resp.DecodeResult(&subscribers)
	if err != nil {
		return nil, err
	}

	return &subscribers, nil
}

// GetSubscriberCredentials returns the authentication credentials.
// Admin or Network Manager role required.
func (c *Client) GetSubscriberCredentials(ctx context.Context, opts *GetSubscriberCredentialsOptions) (*SubscriberCredentials, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/subscribers/" + opts.ID + "/credentials",
	})
	if err != nil {
		return nil, err
	}

	var creds SubscriberCredentials

	err = resp.DecodeResult(&creds)
	if err != nil {
		return nil, err
	}

	return &creds, nil
}
