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
	Description    string `json:"description,omitempty"`
}

type UpdateSubscriberOptions struct {
	ProfileName string `json:"profile_name"`
	Description string `json:"description,omitempty"`
}

type GetSubscriberOptions struct {
	ID string `json:"id"`
}

type DeleteSubscriberOptions struct {
	ID string `json:"id"`
}

// SubscriberStatus is the lightweight status carried in list responses.
type SubscriberStatus struct {
	Registered       bool     `json:"registered"`
	ConnectionState  string   `json:"connection_state,omitempty"`
	RadioAccessTypes []string `json:"radio_access_types,omitempty"`
	NumSessions      int      `json:"num_sessions"`
	LastSeenAt       string   `json:"last_seen_at,omitempty"`
	LastSeenRadio    string   `json:"last_seen_radio,omitempty"`
}

// Subscriber is the summary form returned by ListSubscribers.
type Subscriber struct {
	Imsi        string           `json:"imsi"`
	ProfileName string           `json:"profile_name"`
	Description string           `json:"description,omitempty"`
	Status      SubscriberStatus `json:"status"`
}

type ListSubscribersParams struct {
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	Radio   string `json:"radio,omitempty"`
	Search  string `json:"search,omitempty"`
}

type ListSubscribersResponse struct {
	Items      []Subscriber `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

type UEConnection struct {
	AmfUeNgapID *int64 `json:"amf_ue_ngap_id,omitempty"`
	RanUeNgapID *int64 `json:"ran_ue_ngap_id,omitempty"`
	MMEUeS1apID *int64 `json:"mme_ue_s1ap_id,omitempty"`
	ENBUeS1apID *int64 `json:"enb_ue_s1ap_id,omitempty"`
}

type Registration struct {
	System             string        `json:"system"`
	Registered         bool          `json:"registered"`
	ConnectionState    *string       `json:"connection_state"`
	Radio              string        `json:"radio,omitempty"`
	LastSeenAt         string        `json:"last_seen_at,omitempty"`
	Imei               string        `json:"imei,omitempty"`
	CipheringAlgorithm string        `json:"ciphering_algorithm,omitempty"`
	IntegrityAlgorithm string        `json:"integrity_algorithm,omitempty"`
	Connection         *UEConnection `json:"connection"`
}

// SubscriberDetail is the full form returned by GetSubscriber.
type SubscriberDetail struct {
	Imsi          string         `json:"imsi"`
	ProfileName   string         `json:"profile_name"`
	Description   string         `json:"description,omitempty"`
	Registrations []Registration `json:"registrations"`
	Sessions      []Session      `json:"sessions"`
}

// SessionSlice is the 5GS network slice identifier (S-NSSAI) of a session;
// absent for EPS.
type SessionSlice struct {
	SST int32  `json:"sst"`
	SD  string `json:"sd,omitempty"`
}

// Session is a UE data session — a 5GS PDU session or an EPS PDN connection.
type Session struct {
	System       string        `json:"system"` // "5GS" | "EPS"
	AccessTypes  []string      `json:"access_types"`
	ID           uint8         `json:"id"` // PDU Session ID (5GS) / linked EPS Bearer ID (EPS)
	Status       string        `json:"status"`
	IPType       string        `json:"ip_type,omitempty"` // IPv4 | IPv6 | IPv4v6
	IPv4Address  string        `json:"ipv4_address,omitempty"`
	IPv6Prefix   string        `json:"ipv6_prefix,omitempty"`
	DataNetwork  string        `json:"data_network,omitempty"` // DNN (5GS) / APN (EPS)
	Slice        *SessionSlice `json:"slice,omitempty"`        // 5GS only
	AMBRUplink   string        `json:"ambr_uplink,omitempty"`
	AMBRDownlink string        `json:"ambr_downlink,omitempty"`
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
		Description    string `json:"description,omitempty"`
	}{
		Imsi:           opts.Imsi,
		Key:            opts.Key,
		SequenceNumber: opts.SequenceNumber,
		ProfileName:    opts.ProfileName,
		OPc:            opts.OPc,
		Description:    opts.Description,
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

// UpdateSubscriber replaces a subscriber's profile and description.
func (c *Client) UpdateSubscriber(ctx context.Context, imsi string, opts *UpdateSubscriberOptions) error {
	payload := struct {
		ProfileName string `json:"profile_name"`
		Description string `json:"description,omitempty"`
	}{
		ProfileName: opts.ProfileName,
		Description: opts.Description,
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

	if p.Search != "" {
		query.Set("search", p.Search)
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
