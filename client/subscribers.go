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

type GetSubscriberOptions struct {
	ID string `json:"id"`
}

type DeleteSubscriberOptions struct {
	ID string `json:"id"`
}

// SubscriberStatus is the lightweight status returned by the list endpoint.
type SubscriberStatus struct {
	Registered     bool   `json:"registered"`
	NumPDUSessions int    `json:"num_pdu_sessions"`
	LastSeenAt     string `json:"lastSeenAt,omitempty"`
}

// Subscriber is the summary representation returned by the list endpoint.
type Subscriber struct {
	Imsi        string           `json:"imsi"`
	ProfileName string           `json:"profile_name"`
	Radio       string           `json:"radio,omitempty"`
	Status      SubscriberStatus `json:"status"`
}

// ListSubscribersParams holds the parameters for ListSubscribers.
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

// SubscriberDetailStatus is the rich status returned by the get-single endpoint.
type SubscriberDetailStatus struct {
	Registered         bool   `json:"registered"`
	Imei               string `json:"imei"`
	CipheringAlgorithm string `json:"cipheringAlgorithm"`
	IntegrityAlgorithm string `json:"integrityAlgorithm"`
	LastSeenAt         string `json:"lastSeenAt,omitempty"`
	LastSeenRadio      string `json:"lastSeenRadio,omitempty"`
}

// SubscriberDetail is the full representation returned by the get-single endpoint.
type SubscriberDetail struct {
	Imsi        string                 `json:"imsi"`
	ProfileName string                 `json:"profile_name"`
	Status      SubscriberDetailStatus `json:"status"`
	PDUSessions []SessionInfo          `json:"pdu_sessions"`
}

// SessionInfo is a representation of a PDU session.
type SessionInfo struct {
	PDUSessionID    uint8  `json:"pdu_session_id"`
	Status          string `json:"status"`
	IPAddress       string `json:"ipAddress,omitempty"`
	DNN             string `json:"dnn,omitempty"`
	SST             int32  `json:"sst,omitempty"`
	SD              string `json:"sd,omitempty"`
	SessionAmbrUp   string `json:"session_ambr_uplink,omitempty"`
	SessionAmbrDown string `json:"session_ambr_downlink,omitempty"`
}

// SubscriberCredentials contains the authentication credentials for a subscriber.
type SubscriberCredentials struct {
	Key            string `json:"key"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
}

// GetSubscriberCredentialsOptions holds the parameters for GetSubscriberCredentials.
type GetSubscriberCredentialsOptions struct {
	ID string `json:"id"`
}

// CreateSubscriber creates a new subscriber with the provided options.
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

// GetSubscriber retrieves a subscriber by ID.
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

// DeleteSubscriber deletes a subscriber by ID.
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

// ListSubscribers lists subscribers with pagination and optional filters.
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

// GetSubscriberCredentials retrieves the authentication credentials for a subscriber.
// requires Admin or Network Manager role.
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
