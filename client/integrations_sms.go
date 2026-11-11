// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
)

const smscsPath = "api/v1/integrations/sms/smscs"

type SMSSettings struct {
	ServingNodeNumber string `json:"servingNodeNumber"`
}

type UpdateSMSSettingsOptions struct {
	ServingNodeNumber string
}

type SMSC struct {
	Name           string     `json:"name"`
	ServiceCentres []string   `json:"serviceCentres"`
	Nodes          []SMSCNode `json:"nodes"`
}

type SMSCNode struct {
	OriginHost string              `json:"originHost"`
	Address    string              `json:"address"`
	Port       int                 `json:"port"`
	Status     *DiameterPeerStatus `json:"status,omitempty"`
}

type ListSMSCsResponse struct {
	Items []SMSC `json:"items"`
}

type SMSCNodeOptions struct {
	OriginHost string `json:"originHost"`
	Address    string `json:"address"`
	Port       int    `json:"port,omitempty"`
}

type CreateSMSCOptions struct {
	Name           string
	ServiceCentres []string
	Nodes          []SMSCNodeOptions
}

type UpdateSMSCOptions struct {
	ServiceCentres []string
	Nodes          []SMSCNodeOptions
}

func (c *Client) GetSMSSettings(ctx context.Context) (*SMSSettings, error) {
	var settings SMSSettings

	if err := c.integrationsRequest(ctx, "GET", "api/v1/integrations/sms", nil, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func (c *Client) UpdateSMSSettings(ctx context.Context, opts *UpdateSMSSettingsOptions) error {
	return c.integrationsRequest(ctx, "PUT", "api/v1/integrations/sms", SMSSettings{ServingNodeNumber: opts.ServingNodeNumber}, nil)
}

func (c *Client) ListSMSCs(ctx context.Context) (*ListSMSCsResponse, error) {
	var smscs ListSMSCsResponse

	if err := c.integrationsRequest(ctx, "GET", smscsPath, nil, &smscs); err != nil {
		return nil, err
	}

	return &smscs, nil
}

func (c *Client) GetSMSC(ctx context.Context, name string) (*SMSC, error) {
	var smsc SMSC

	if err := c.integrationsRequest(ctx, "GET", smscsPath+"/"+url.PathEscape(name), nil, &smsc); err != nil {
		return nil, err
	}

	return &smsc, nil
}

func (c *Client) CreateSMSC(ctx context.Context, opts *CreateSMSCOptions) (*SMSC, error) {
	payload := struct {
		Name           string            `json:"name"`
		ServiceCentres []string          `json:"serviceCentres"`
		Nodes          []SMSCNodeOptions `json:"nodes"`
	}{
		Name:           opts.Name,
		ServiceCentres: opts.ServiceCentres,
		Nodes:          opts.Nodes,
	}

	var smsc SMSC

	if err := c.integrationsRequest(ctx, "POST", smscsPath, payload, &smsc); err != nil {
		return nil, err
	}

	return &smsc, nil
}

func (c *Client) UpdateSMSC(ctx context.Context, name string, opts *UpdateSMSCOptions) error {
	payload := struct {
		ServiceCentres []string          `json:"serviceCentres"`
		Nodes          []SMSCNodeOptions `json:"nodes"`
	}{
		ServiceCentres: opts.ServiceCentres,
		Nodes:          opts.Nodes,
	}

	return c.integrationsRequest(ctx, "PUT", smscsPath+"/"+url.PathEscape(name), payload, nil)
}

func (c *Client) DeleteSMSC(ctx context.Context, name string) error {
	return c.integrationsRequest(ctx, "DELETE", smscsPath+"/"+url.PathEscape(name), nil, nil)
}

func (c *Client) integrationsRequest(ctx context.Context, method, path string, payload any, result any) error {
	opts := &RequestOptions{
		Type:   SyncRequest,
		Method: method,
		Path:   path,
	}

	if payload != nil {
		var body bytes.Buffer

		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return err
		}

		opts.Body = &body
	}

	resp, err := c.Requester.Do(ctx, opts)
	if err != nil {
		return err
	}

	if result == nil {
		return nil
	}

	return resp.DecodeResult(result)
}
