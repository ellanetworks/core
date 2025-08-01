package client

import (
	"bytes"
	"context"
	"encoding/json"
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

type Subscriber struct {
	Imsi           string `json:"imsi"`
	IPAddress      string `json:"ipAddress"`
	Opc            string `json:"opc"`
	SequenceNumber string `json:"sequenceNumber"`
	Key            string `json:"key"`
	PolicyName     string `json:"policyName"`
}

func (c *Client) CreateSubscriber(opts *CreateSubscriberOptions) error {
	payload := struct {
		Imsi           string `json:"imsi"`
		Key            string `json:"key"`
		SequenceNumber string `json:"sequenceNumber"`
		PolicyName     string `json:"policyName"`
	}{
		Imsi:           opts.Imsi,
		Key:            opts.Key,
		SequenceNumber: opts.SequenceNumber,
		PolicyName:     opts.PolicyName,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(context.Background(), &RequestOptions{
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

func (c *Client) GetSubscriber(opts *GetSubscriberOptions) (*Subscriber, error) {
	resp, err := c.Requester.Do(context.Background(), &RequestOptions{
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

func (c *Client) DeleteSubscriber(opts *DeleteSubscriberOptions) error {
	_, err := c.Requester.Do(context.Background(), &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/subscribers/" + opts.ID,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ListSubscribers() ([]*Subscriber, error) {
	resp, err := c.Requester.Do(context.Background(), &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/subscribers",
	})
	if err != nil {
		return nil, err
	}
	var subscribers []*Subscriber
	err = resp.DecodeResult(&subscribers)
	if err != nil {
		return nil, err
	}
	return subscribers, nil
}
