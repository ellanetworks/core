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
	PolicyName     string `json:"policyName"`
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
	Registered bool   `json:"registered"`
	IPAddress  string `json:"ipAddress"`
}

// Subscriber is the summary representation returned by the list endpoint.
type Subscriber struct {
	Imsi       string           `json:"imsi"`
	PolicyName string           `json:"policyName"`
	Status     SubscriberStatus `json:"status"`
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
	IPAddress          string `json:"ipAddress"`
	Imei               string `json:"imei"`
	CipheringAlgorithm string `json:"cipheringAlgorithm"`
	IntegrityAlgorithm string `json:"integrityAlgorithm"`
	LastSeenAt         string `json:"lastSeenAt,omitempty"`
	LastSeenRadio      string `json:"lastSeenRadio,omitempty"`
}

// SubscriberDetail is the full representation returned by the get-single endpoint.
type SubscriberDetail struct {
	Imsi       string                 `json:"imsi"`
	PolicyName string                 `json:"policyName"`
	Status     SubscriberDetailStatus `json:"status"`
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
		PolicyName     string `json:"policyName"`
		OPc            string `json:"opc,omitempty"`
	}{
		Imsi:           opts.Imsi,
		Key:            opts.Key,
		SequenceNumber: opts.SequenceNumber,
		PolicyName:     opts.PolicyName,
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

// ListSubscribers lists subscribers with pagination.
func (c *Client) ListSubscribers(ctx context.Context, p *ListParams) (*ListSubscribersResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/subscribers",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
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
