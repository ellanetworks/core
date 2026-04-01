package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type PolicyRule struct {
	Description  string  `json:"description"`
	RemotePrefix *string `json:"remote_prefix,omitempty"`
	Protocol     int32   `json:"protocol"`
	PortLow      int32   `json:"port_low"`
	PortHigh     int32   `json:"port_high"`
	Action       string  `json:"action"`
}

type PolicyRules struct {
	Uplink   []PolicyRule `json:"uplink,omitempty"`
	Downlink []PolicyRule `json:"downlink,omitempty"`
}

type CreatePolicyOptions struct {
	Name            string       `json:"name"`
	BitrateUplink   string       `json:"bitrate_uplink"`
	BitrateDownlink string       `json:"bitrate_downlink"`
	Var5qi          int32        `json:"var5qi"`
	Arp             int32        `json:"arp"`
	DataNetworkName string       `json:"data_network_name"`
	Rules           *PolicyRules `json:"rules,omitempty"`
}

type UpdatePolicyOptions struct {
	Name            string       `json:"name,omitempty"`
	BitrateUplink   string       `json:"bitrate_uplink,omitempty"`
	BitrateDownlink string       `json:"bitrate_downlink,omitempty"`
	Var5qi          int32        `json:"var5qi,omitempty"`
	Arp             int32        `json:"arp,omitempty"`
	DataNetworkName string       `json:"data_network_name,omitempty"`
	Rules           *PolicyRules `json:"rules,omitempty"`
}

type GetPolicyOptions struct {
	Name string `json:"name"`
}

type DeletePolicyOptions struct {
	Name string `json:"name"`
}

type Policy struct {
	Name            string       `json:"name"`
	BitrateUplink   string       `json:"bitrate_uplink"`
	BitrateDownlink string       `json:"bitrate_downlink"`
	Var5qi          int32        `json:"var5qi"`
	Arp             int32        `json:"arp"`
	DataNetworkName string       `json:"data_network_name"`
	Rules           *PolicyRules `json:"rules,omitempty"`
}

type ListPoliciesResponse struct {
	Items      []Policy `json:"items"`
	Page       int      `json:"page"`
	PerPage    int      `json:"per_page"`
	TotalCount int      `json:"total_count"`
}

// CreatePolicy creates a new policy with the provided options.
// Optionally includes network rules organized by direction (uplink/downlink).
// Rules are created in the order they are provided.
func (c *Client) CreatePolicy(ctx context.Context, opts *CreatePolicyOptions) error {
	payload := struct {
		Name            string       `json:"name"`
		BitrateUplink   string       `json:"bitrate_uplink"`
		BitrateDownlink string       `json:"bitrate_downlink"`
		Var5qi          int32        `json:"var5qi"`
		Arp             int32        `json:"arp"`
		DataNetworkName string       `json:"data_network_name"`
		Rules           *PolicyRules `json:"rules,omitempty"`
	}{
		Name:            opts.Name,
		BitrateUplink:   opts.BitrateUplink,
		BitrateDownlink: opts.BitrateDownlink,
		Var5qi:          opts.Var5qi,
		Arp:             opts.Arp,
		DataNetworkName: opts.DataNetworkName,
		Rules:           opts.Rules,
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

// GetPolicy retrieves a policy by name, including any associated network rules.
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

// UpdatePolicy updates an existing policy by name.
// Optionally updates network rules by providing a rules object.
// If rules are provided, existing rules are deleted and replaced with new ones.
// To delete all rules while keeping the policy, provide empty rule arrays: {"uplink": [], "downlink": []}.
// Omit the rules field entirely to keep existing rules unchanged.
func (c *Client) UpdatePolicy(ctx context.Context, name string, opts *UpdatePolicyOptions) error {
	payload := struct {
		BitrateUplink   string       `json:"bitrate_uplink,omitempty"`
		BitrateDownlink string       `json:"bitrate_downlink,omitempty"`
		Var5qi          int32        `json:"var5qi,omitempty"`
		Arp             int32        `json:"arp,omitempty"`
		DataNetworkName string       `json:"data_network_name,omitempty"`
		Rules           *PolicyRules `json:"rules,omitempty"`
	}{
		BitrateUplink:   opts.BitrateUplink,
		BitrateDownlink: opts.BitrateDownlink,
		Var5qi:          opts.Var5qi,
		Arp:             opts.Arp,
		DataNetworkName: opts.DataNetworkName,
		Rules:           opts.Rules,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/policies/" + name,
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// DeletePolicy deletes a policy by name.
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

// ListPolicies lists policies with pagination.
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
