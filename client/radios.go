package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type GetRadioOptions struct {
	Name string `json:"name"`
}

type PlmnID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type Tai struct {
	PlmnID PlmnID `json:"plmnID"`
	Tac    string `json:"tac"`
}

type Snssai struct {
	Sst int32  `json:"sst"`
	Sd  string `json:"sd"`
}

type SupportedTAI struct {
	Tai     Tai      `json:"tai"`
	SNssais []Snssai `json:"snssais"`
}

type Radio struct {
	Name          string         `json:"name"`
	ID            string         `json:"id"`
	Address       string         `json:"address"`
	SupportedTAIs []SupportedTAI `json:"supported_tais"`
}

type ListRadiosResponse struct {
	Items      []Radio `json:"items"`
	Page       int     `json:"page"`
	PerPage    int     `json:"per_page"`
	TotalCount int     `json:"total_count"`
}

type GetRadioEventsRetentionPolicy struct {
	Days int `json:"days"`
}

type UpdateRadioEventsRetentionPolicyOptions struct {
	Days int `json:"days"`
}

type RadioEvent struct {
	ID          int    `json:"id"`
	Timestamp   string `json:"timestamp"`
	Level       string `json:"level"`
	Protocol    string `json:"protocol"`
	MessageType string `json:"message_type"`
	Direction   string `json:"direction"`
	Raw         string `json:"raw"`
	Details     string `json:"details"`
}

type ListRadioEventsResponse struct {
	Items      []RadioEvent `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

type ListRadioEventsParams struct {
	Page          int    `json:"page"`
	PerPage       int    `json:"per_page"`
	Protocol      string `json:"protocol"`
	Direction     string `json:"direction"`
	MessageType   string `json:"message_type"`
	TimestampFrom string `json:"timestamp_from"`
	TimestampTo   string `json:"timestamp_to"`
}

type RadioEventContent struct {
	Decoded any    `json:"decoded"`
	Raw     string `json:"raw"`
}

func (c *Client) GetRadio(ctx context.Context, opts *GetRadioOptions) (*Radio, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/ran/radios/" + opts.Name,
	})
	if err != nil {
		return nil, err
	}

	var radioResponse Radio

	err = resp.DecodeResult(&radioResponse)
	if err != nil {
		return nil, err
	}
	return &radioResponse, nil
}

func (c *Client) ListRadios(ctx context.Context, p *ListParams) (*ListRadiosResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/ran/radios",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var radios ListRadiosResponse

	err = resp.DecodeResult(&radios)
	if err != nil {
		return nil, err
	}

	return &radios, nil
}

func (c *Client) ListRadioEvents(ctx context.Context, p *ListRadioEventsParams) (*ListRadioEventsResponse, error) {
	query := url.Values{}
	if p.Page != 0 {
		query.Set("page", fmt.Sprintf("%d", p.Page))
	}

	if p.PerPage != 0 {
		query.Set("per_page", fmt.Sprintf("%d", p.PerPage))
	}

	if p.Protocol != "" {
		query.Set("protocol", p.Protocol)
	}

	if p.Direction != "" {
		query.Set("direction", p.Direction)
	}

	if p.MessageType != "" {
		query.Set("message_type", p.MessageType)
	}

	if p.TimestampFrom != "" {
		query.Set("timestamp_from", p.TimestampFrom)
	}

	if p.TimestampTo != "" {
		query.Set("timestamp_to", p.TimestampTo)
	}

	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/ran/events",
		Query:  query,
	})
	if err != nil {
		return nil, err
	}

	var radioEvents ListRadioEventsResponse

	err = resp.DecodeResult(&radioEvents)
	if err != nil {
		return nil, err
	}

	return &radioEvents, nil
}

func (c *Client) ClearRadioEvents(ctx context.Context) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/logs/network",
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetRadioEventRetentionPolicy(ctx context.Context) (*GetRadioEventsRetentionPolicy, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/logs/network/retention",
	})
	if err != nil {
		return nil, err
	}

	var policy GetRadioEventsRetentionPolicy

	err = resp.DecodeResult(&policy)
	if err != nil {
		return nil, err
	}

	return &policy, nil
}

func (c *Client) UpdateRadioEventRetentionPolicy(ctx context.Context, opts *UpdateRadioEventsRetentionPolicyOptions) error {
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
		Path:   "api/v1/logs/network/retention",
		Body:   &body,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetRadioEvent(ctx context.Context, id int) (*RadioEventContent, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   fmt.Sprintf("api/v1/logs/network/%d", id),
	})
	if err != nil {
		return nil, err
	}

	var logContent RadioEventContent

	err = resp.DecodeResult(&logContent)
	if err != nil {
		return nil, err
	}

	return &logContent, nil
}
