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

type SubscriberSession struct {
	IPAddress string `json:"ipAddress"`
}

type SubscriberStatus struct {
	Registered bool                `json:"registered"`
	Sessions   []SubscriberSession `json:"sessions"`
}

type Subscriber struct {
	Imsi           string           `json:"imsi"`
	Opc            string           `json:"opc"`
	SequenceNumber string           `json:"sequenceNumber"`
	Key            string           `json:"key"`
	PolicyName     string           `json:"policyName"`
	Status         SubscriberStatus `json:"status"`
}

type ListSubscribersResponse struct {
	Items      []Subscriber `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

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

func (c *Client) GetSubscriber(ctx context.Context, opts *GetSubscriberOptions) (*Subscriber, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/subscribers/" + opts.ID,
	})
	if err != nil {
		return nil, err
	}

	var subscriberResponse Subscriber

	err = resp.DecodeResult(&subscriberResponse)
	if err != nil {
		return nil, err
	}

	return &subscriberResponse, nil
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

// http://127.0.0.1:5002/api/v1/subscribers?page=1&per_page=25

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
