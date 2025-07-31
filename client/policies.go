package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type CreatePolicyOptions struct {
	Name            string `json:"name"`
	IPPool          string `json:"ip-pool"`
	DNS             string `json:"dns"`
	Mtu             int32  `json:"mtu"`
	BitrateUplink   string `json:"bitrate-uplink"`
	BitrateDownlink string `json:"bitrate-downlink"`
	Var5qi          int32  `json:"var5qi"`
	PriorityLevel   int32  `json:"priority-level"`
}

type GetPolicyOptions struct {
	Name string `json:"name"`
}

type DeletePolicyOptions struct {
	Name string `json:"name"`
}

type Policy struct {
	Name            string `json:"name"`
	IPPool          string `json:"ip-pool"`
	DNS             string `json:"dns"`
	Mtu             int32  `json:"mtu"`
	BitrateUplink   string `json:"bitrate-uplink"`
	BitrateDownlink string `json:"bitrate-downlink"`
	Var5qi          int32  `json:"var5qi"`
	PriorityLevel   int32  `json:"priority-level"`
}

func (c *Client) CreatePolicy(opts *CreatePolicyOptions) error {
	payload := struct {
		Name            string `json:"name"`
		IPPool          string `json:"ip-pool"`
		DNS             string `json:"dns"`
		Mtu             int32  `json:"mtu"`
		BitrateUplink   string `json:"bitrate-uplink"`
		BitrateDownlink string `json:"bitrate-downlink"`
		Var5qi          int32  `json:"var5qi"`
		PriorityLevel   int32  `json:"priority-level"`
	}{
		Name:            opts.Name,
		IPPool:          opts.IPPool,
		DNS:             opts.DNS,
		Mtu:             opts.Mtu,
		BitrateUplink:   opts.BitrateUplink,
		BitrateDownlink: opts.BitrateDownlink,
		Var5qi:          opts.Var5qi,
		PriorityLevel:   opts.PriorityLevel,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(context.Background(), &RequestOptions{
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

func (c *Client) GetPolicy(opts *GetPolicyOptions) (*Policy, error) {
	resp, err := c.Requester.Do(context.Background(), &RequestOptions{
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

func (c *Client) DeletePolicy(opts *DeletePolicyOptions) error {
	_, err := c.Requester.Do(context.Background(), &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/policies/" + opts.Name,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ListPolicies() ([]*Policy, error) {
	resp, err := c.Requester.Do(context.Background(), &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/policies",
	})
	if err != nil {
		return nil, err
	}
	var policies []*Policy
	err = resp.DecodeResult(&policies)
	if err != nil {
		return nil, err
	}
	return policies, nil
}
