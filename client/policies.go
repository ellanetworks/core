package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type CreatePolicyOptions struct {
	Name            string `json:"name"`
	BitrateUplink   string `json:"bitrate_uplink"`
	BitrateDownlink string `json:"bitrate_downlink"`
	Var5qi          int32  `json:"var5qi"`
	Arp             int32  `json:"arp"`
	DataNetworkName string `json:"data_network_name"`
}

type GetPolicyOptions struct {
	Name string `json:"name"`
}

type DeletePolicyOptions struct {
	Name string `json:"name"`
}

type Policy struct {
	Name            string `json:"name"`
	BitrateUplink   string `json:"bitrate_uplink"`
	BitrateDownlink string `json:"bitrate_downlink"`
	Var5qi          int32  `json:"var5qi"`
	Arp             int32  `json:"arp"`
	DataNetworkName string `json:"data_network_name"`
}

type ListPoliciesResponse struct {
	Items      []Policy `json:"items"`
	Page       int      `json:"page"`
	PerPage    int      `json:"per_page"`
	TotalCount int      `json:"total_count"`
}

func (c *Client) CreatePolicy(ctx context.Context, opts *CreatePolicyOptions) error {
	payload := struct {
		Name            string `json:"name"`
		BitrateUplink   string `json:"bitrate_uplink"`
		BitrateDownlink string `json:"bitrate_downlink"`
		Var5qi          int32  `json:"var5qi"`
		Arp             int32  `json:"arp"`
		DataNetworkName string `json:"data_network_name"`
	}{
		Name:            opts.Name,
		BitrateUplink:   opts.BitrateUplink,
		BitrateDownlink: opts.BitrateDownlink,
		Var5qi:          opts.Var5qi,
		Arp:             opts.Arp,
		DataNetworkName: opts.DataNetworkName,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/policies",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) GetPolicy(ctx context.Context, opts *GetPolicyOptions) (*Policy, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/policies/" + opts.Name,
	})
	if err != nil {
		return nil, err
	}

	var policyResponse Policy

	err = resp.DecodeResult(&policyResponse)
	if err != nil {
		return nil, err
	}

	return &policyResponse, nil
}

func (c *Client) DeletePolicy(ctx context.Context, opts *DeletePolicyOptions) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/policies/" + opts.Name,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) ListPolicies(ctx context.Context, p *ListParams) (*ListPoliciesResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/policies",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var policies ListPoliciesResponse

	err = resp.DecodeResult(&policies)
	if err != nil {
		return nil, err
	}

	return &policies, nil
}
