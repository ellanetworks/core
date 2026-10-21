// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
)

type IMSApplicationServer struct {
	Name                    string   `json:"name"`
	ServerName              string   `json:"server_name"`
	PublicServiceIdentities []string `json:"public_service_identities"`
}

type ListIMSApplicationServersResponse struct {
	Items []IMSApplicationServer `json:"items"`
}

type CreateIMSApplicationServerOptions struct {
	Name                    string   `json:"name"`
	ServerName              string   `json:"server_name"`
	PublicServiceIdentities []string `json:"public_service_identities"`
}

type UpdateIMSApplicationServerOptions struct {
	ServerName              string   `json:"server_name"`
	PublicServiceIdentities []string `json:"public_service_identities"`
}

type IMSHeaderMatch struct {
	Header  string  `json:"header"`
	Content *string `json:"content,omitempty"`
}

type IMSSessionDescription struct {
	Line    string  `json:"line"`
	Content *string `json:"content,omitempty"`
}

type IMSServicePointTrigger struct {
	ConditionNegated   bool                   `json:"condition_negated,omitempty"`
	Group              []int32                `json:"group"`
	RequestURI         *string                `json:"request_uri,omitempty"`
	Method             *string                `json:"method,omitempty"`
	SIPHeader          *IMSHeaderMatch        `json:"sip_header,omitempty"`
	SessionCase        *string                `json:"session_case,omitempty"`
	SessionDescription *IMSSessionDescription `json:"session_description,omitempty"`
	RegistrationType   []string               `json:"registration_type,omitempty"`
}

type IMSTriggerPoint struct {
	ConditionTypeCNF bool                     `json:"condition_type_cnf"`
	SPT              []IMSServicePointTrigger `json:"spt"`
}

type IMSInitialFilterCriterion struct {
	Priority                int             `json:"priority"`
	TriggerPoint            IMSTriggerPoint `json:"trigger_point"`
	DefaultHandling         string          `json:"default_handling"`
	IncludeRegisterRequest  bool            `json:"include_register_request"`
	IncludeRegisterResponse bool            `json:"include_register_response"`
}

type IMSService struct {
	Name                  string                      `json:"name"`
	Kind                  string                      `json:"kind"`
	ApplicationServerName string                      `json:"application_server_name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

type ListIMSServicesResponse struct {
	Items []IMSService `json:"items"`
}

type CreateIMSServiceOptions struct {
	Name                  string                      `json:"name"`
	Kind                  string                      `json:"kind"`
	ApplicationServerName string                      `json:"application_server_name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria,omitempty"`
}

type UpdateIMSServiceOptions struct {
	Kind                  string                      `json:"kind"`
	ApplicationServerName string                      `json:"application_server_name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria,omitempty"`
}

type ProfileIMSServices struct {
	Services []string `json:"services"`
}

const (
	imsApplicationServersPath = "api/v1/operator/ims/application-servers"
	imsServicesPath           = "api/v1/operator/ims/services"
)

func (c *Client) imsRequest(ctx context.Context, method, path string, payload any, result any) error {
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

func (c *Client) ListIMSApplicationServers(ctx context.Context) (*ListIMSApplicationServersResponse, error) {
	var list ListIMSApplicationServersResponse

	if err := c.imsRequest(ctx, "GET", imsApplicationServersPath, nil, &list); err != nil {
		return nil, err
	}

	return &list, nil
}

func (c *Client) GetIMSApplicationServer(ctx context.Context, name string) (*IMSApplicationServer, error) {
	var as IMSApplicationServer

	if err := c.imsRequest(ctx, "GET", imsApplicationServersPath+"/"+url.PathEscape(name), nil, &as); err != nil {
		return nil, err
	}

	return &as, nil
}

func (c *Client) CreateIMSApplicationServer(ctx context.Context, opts *CreateIMSApplicationServerOptions) (*IMSApplicationServer, error) {
	payload := *opts
	if payload.PublicServiceIdentities == nil {
		payload.PublicServiceIdentities = []string{}
	}

	var as IMSApplicationServer

	if err := c.imsRequest(ctx, "POST", imsApplicationServersPath, payload, &as); err != nil {
		return nil, err
	}

	return &as, nil
}

func (c *Client) UpdateIMSApplicationServer(ctx context.Context, name string, opts *UpdateIMSApplicationServerOptions) error {
	payload := *opts
	if payload.PublicServiceIdentities == nil {
		payload.PublicServiceIdentities = []string{}
	}

	return c.imsRequest(ctx, "PUT", imsApplicationServersPath+"/"+url.PathEscape(name), payload, nil)
}

func (c *Client) DeleteIMSApplicationServer(ctx context.Context, name string) error {
	return c.imsRequest(ctx, "DELETE", imsApplicationServersPath+"/"+url.PathEscape(name), nil, nil)
}

func (c *Client) ListIMSServices(ctx context.Context) (*ListIMSServicesResponse, error) {
	var list ListIMSServicesResponse

	if err := c.imsRequest(ctx, "GET", imsServicesPath, nil, &list); err != nil {
		return nil, err
	}

	return &list, nil
}

func (c *Client) GetIMSService(ctx context.Context, name string) (*IMSService, error) {
	var svc IMSService

	if err := c.imsRequest(ctx, "GET", imsServicesPath+"/"+url.PathEscape(name), nil, &svc); err != nil {
		return nil, err
	}

	return &svc, nil
}

func (c *Client) CreateIMSService(ctx context.Context, opts *CreateIMSServiceOptions) (*IMSService, error) {
	var svc IMSService

	if err := c.imsRequest(ctx, "POST", imsServicesPath, opts, &svc); err != nil {
		return nil, err
	}

	return &svc, nil
}

func (c *Client) UpdateIMSService(ctx context.Context, name string, opts *UpdateIMSServiceOptions) error {
	return c.imsRequest(ctx, "PUT", imsServicesPath+"/"+url.PathEscape(name), opts, nil)
}

func (c *Client) DeleteIMSService(ctx context.Context, name string) error {
	return c.imsRequest(ctx, "DELETE", imsServicesPath+"/"+url.PathEscape(name), nil, nil)
}

func (c *Client) GetProfileIMSServices(ctx context.Context, profileName string) (*ProfileIMSServices, error) {
	var services ProfileIMSServices

	if err := c.imsRequest(ctx, "GET", "api/v1/profiles/"+url.PathEscape(profileName)+"/ims-services", nil, &services); err != nil {
		return nil, err
	}

	return &services, nil
}

func (c *Client) SetProfileIMSServices(ctx context.Context, profileName string, services []string) error {
	payload := ProfileIMSServices{Services: services}
	if payload.Services == nil {
		payload.Services = []string{}
	}

	return c.imsRequest(ctx, "PUT", "api/v1/profiles/"+url.PathEscape(profileName)+"/ims-services", payload, nil)
}
