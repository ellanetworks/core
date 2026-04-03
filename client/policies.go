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

// CreatePolicyOptions contains the parameters for creating a new policy.
type CreatePolicyOptions struct {
	Name            string `json:"name"`
	ProfileName     string `json:"profile_name"`
	SliceName       string `json:"slice_name"`
	DataNetworkName string `json:"data_network_name"`
	// SessionAmbrUplink is the maximum uplink bitrate for a single PDU session.
	// Enforced by Ella Core. Example: "100 Mbps".
	SessionAmbrUplink string `json:"session_ambr_uplink"`
	// SessionAmbrDownlink is the maximum downlink bitrate for a single PDU session.
	// Enforced by Ella Core. Example: "200 Mbps".
	SessionAmbrDownlink string `json:"session_ambr_downlink"`
	// Var5qi is the 5G QoS Identifier, signaled to the radio for scheduling.
	// Non-GBR values only: 5, 6, 7, 8, 9, 69, 70, 79, 80.
	Var5qi int32 `json:"var5qi"`
	// Arp is the Allocation and Retention Priority (1–15). Used at session setup
	// for admission control and pre-emption; 1 = highest priority.
	Arp   int32        `json:"arp"`
	Rules *PolicyRules `json:"rules,omitempty"`
}

type UpdatePolicyOptions struct {
	ProfileName         string `json:"profile_name,omitempty"`
	SliceName           string `json:"slice_name,omitempty"`
	DataNetworkName     string `json:"data_network_name,omitempty"`
	SessionAmbrUplink   string `json:"session_ambr_uplink,omitempty"`
	SessionAmbrDownlink string `json:"session_ambr_downlink,omitempty"`
	// Var5qi is the 5G QoS Identifier, signaled to the radio for scheduling.
	// Non-GBR values only: 5, 6, 7, 8, 9, 69, 70, 79, 80.
	Var5qi int32 `json:"var5qi,omitempty"`
	// Arp is the Allocation and Retention Priority (1–15). Used at session setup
	// for admission control and pre-emption; 1 = highest priority.
	Arp   int32        `json:"arp,omitempty"`
	Rules *PolicyRules `json:"rules,omitempty"`
}

type GetPolicyOptions struct {
	Name string `json:"name"`
}

type DeletePolicyOptions struct {
	Name string `json:"name"`
}

// Policy represents a QoS policy for a specific (profile, slice, data network) combination.
// Session AMBR caps the bitrate of a single PDU session and is enforced by Ella Core.
type Policy struct {
	Name                string `json:"name"`
	ProfileName         string `json:"profile_name"`
	SliceName           string `json:"slice_name"`
	DataNetworkName     string `json:"data_network_name"`
	SessionAmbrUplink   string `json:"session_ambr_uplink"`
	SessionAmbrDownlink string `json:"session_ambr_downlink"`
	// Var5qi is the 5G QoS Identifier, signaled to the radio for scheduling.
	// Non-GBR values only: 5, 6, 7, 8, 9, 69, 70, 79, 80.
	Var5qi int32 `json:"var5qi"`
	// Arp is the Allocation and Retention Priority (1–15). Used at session setup
	// for admission control and pre-emption; 1 = highest priority.
	Arp   int32        `json:"arp"`
	Rules *PolicyRules `json:"rules,omitempty"`
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
		Name                string       `json:"name"`
		ProfileName         string       `json:"profile_name"`
		SliceName           string       `json:"slice_name"`
		DataNetworkName     string       `json:"data_network_name"`
		SessionAmbrUplink   string       `json:"session_ambr_uplink"`
		SessionAmbrDownlink string       `json:"session_ambr_downlink"`
		Var5qi              int32        `json:"var5qi"`
		Arp                 int32        `json:"arp"`
		Rules               *PolicyRules `json:"rules,omitempty"`
	}{
		Name:                opts.Name,
		ProfileName:         opts.ProfileName,
		SliceName:           opts.SliceName,
		DataNetworkName:     opts.DataNetworkName,
		SessionAmbrUplink:   opts.SessionAmbrUplink,
		SessionAmbrDownlink: opts.SessionAmbrDownlink,
		Var5qi:              opts.Var5qi,
		Arp:                 opts.Arp,
		Rules:               opts.Rules,
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
// Existing network rules are always replaced on every update.
// If Rules is nil (omitted), all existing rules are deleted.
// To keep existing rules, re-supply them in opts.Rules.
// To set specific rules, provide them in opts.Rules; existing rules are deleted and replaced.
func (c *Client) UpdatePolicy(ctx context.Context, name string, opts *UpdatePolicyOptions) error {
	payload := struct {
		ProfileName         string       `json:"profile_name,omitempty"`
		SliceName           string       `json:"slice_name,omitempty"`
		DataNetworkName     string       `json:"data_network_name,omitempty"`
		SessionAmbrUplink   string       `json:"session_ambr_uplink,omitempty"`
		SessionAmbrDownlink string       `json:"session_ambr_downlink,omitempty"`
		Var5qi              int32        `json:"var5qi,omitempty"`
		Arp                 int32        `json:"arp,omitempty"`
		Rules               *PolicyRules `json:"rules,omitempty"`
	}{
		ProfileName:         opts.ProfileName,
		SliceName:           opts.SliceName,
		DataNetworkName:     opts.DataNetworkName,
		SessionAmbrUplink:   opts.SessionAmbrUplink,
		SessionAmbrDownlink: opts.SessionAmbrDownlink,
		Var5qi:              opts.Var5qi,
		Arp:                 opts.Arp,
		Rules:               opts.Rules,
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
