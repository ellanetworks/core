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
	Name            string `json:"name"`
	ProfileName     string `json:"profile_name"`
	SliceName       string `json:"slice_name"`
	DataNetworkName string `json:"data_network_name"`
	// SessionAmbrUplink caps the uplink bitrate of a single PDU session,
	// e.g. "100 Mbps".
	SessionAmbrUplink string `json:"session_ambr_uplink"`
	// SessionAmbrDownlink caps the downlink bitrate of a single PDU session,
	// e.g. "200 Mbps".
	SessionAmbrDownlink string `json:"session_ambr_downlink"`
	// Var5qi is the 5G QoS Identifier. Non-GBR values only: 5, 6, 7, 8, 9,
	// 69, 70, 79, 80.
	Var5qi int32 `json:"var5qi"`
	// Arp is the Allocation and Retention Priority (1–15, 1 = highest).
	Arp   int32        `json:"arp"`
	Rules *PolicyRules `json:"rules,omitempty"`
}

type UpdatePolicyOptions struct {
	ProfileName         string `json:"profile_name,omitempty"`
	SliceName           string `json:"slice_name,omitempty"`
	DataNetworkName     string `json:"data_network_name,omitempty"`
	SessionAmbrUplink   string `json:"session_ambr_uplink,omitempty"`
	SessionAmbrDownlink string `json:"session_ambr_downlink,omitempty"`
	// Var5qi is the 5G QoS Identifier. Non-GBR values only: 5, 6, 7, 8, 9,
	// 69, 70, 79, 80.
	Var5qi int32 `json:"var5qi,omitempty"`
	// Arp is the Allocation and Retention Priority (1–15, 1 = highest).
	Arp   int32        `json:"arp,omitempty"`
	Rules *PolicyRules `json:"rules,omitempty"`
}

type GetPolicyOptions struct {
	Name string `json:"name"`
}

type DeletePolicyOptions struct {
	Name string `json:"name"`
}

// Policy is a QoS policy bound to a specific (profile, slice, data network)
// combination. Session AMBR caps the bitrate of a single PDU session.
type Policy struct {
	Name                string `json:"name"`
	ProfileName         string `json:"profile_name"`
	SliceName           string `json:"slice_name"`
	DataNetworkName     string `json:"data_network_name"`
	SessionAmbrUplink   string `json:"session_ambr_uplink"`
	SessionAmbrDownlink string `json:"session_ambr_downlink"`
	// Var5qi is the 5G QoS Identifier. Non-GBR values only: 5, 6, 7, 8, 9,
	// 69, 70, 79, 80.
	Var5qi int32 `json:"var5qi"`
	// Arp is the Allocation and Retention Priority (1–15, 1 = highest).
	Arp   int32        `json:"arp"`
	Rules *PolicyRules `json:"rules,omitempty"`
}

type ListPoliciesResponse struct {
	Items      []Policy `json:"items"`
	Page       int      `json:"page"`
	PerPage    int      `json:"per_page"`
	TotalCount int      `json:"total_count"`
}

// CreatePolicy creates a policy, optionally with network rules. Rules are
// applied in the order they appear in opts.Rules.
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

// GetPolicy retrieves a policy by name. The response includes the
// associated network rules; ListPolicies omits them.
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

// UpdatePolicy replaces an existing policy by name. Network rules are
// destructively replaced on every update: if opts.Rules is nil, all
// existing rules are deleted. To keep them, re-supply the current list.
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
