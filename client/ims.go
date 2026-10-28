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

type IMSCondition struct {
	Negated            bool                   `json:"negated,omitempty"`
	RequestURI         *string                `json:"request_uri,omitempty"`
	Method             *string                `json:"method,omitempty"`
	RegistrationTypes  []string               `json:"registration_types,omitempty"`
	SIPHeader          *IMSHeaderMatch        `json:"sip_header,omitempty"`
	SessionCase        *string                `json:"session_case,omitempty"`
	SessionDescription *IMSSessionDescription `json:"session_description,omitempty"`
}

type IMSConditionGroup struct {
	Conditions []IMSCondition `json:"conditions"`
}

type IMSTrigger struct {
	ConditionsMatch string              `json:"conditions_match"`
	Groups          []IMSConditionGroup `json:"groups"`
}

type IMSInitialFilterCriterion struct {
	Priority                int         `json:"priority"`
	ApplicationServerName   string      `json:"application_server_name"`
	Trigger                 *IMSTrigger `json:"trigger,omitempty"`
	ProfilePart             *string     `json:"profile_part,omitempty"`
	DefaultHandling         *string     `json:"default_handling,omitempty"`
	ServiceInfo             *string     `json:"service_info,omitempty"`
	IncludeRegisterRequest  bool        `json:"include_register_request"`
	IncludeRegisterResponse bool        `json:"include_register_response"`
}

type IMSServiceProfile struct {
	Name                  string                      `json:"name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

type ListIMSServiceProfilesResponse struct {
	Items []IMSServiceProfile `json:"items"`
}

type CreateIMSServiceProfileOptions struct {
	Name                  string                      `json:"name"`
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

type UpdateIMSServiceProfileOptions struct {
	InitialFilterCriteria []IMSInitialFilterCriterion `json:"initial_filter_criteria"`
}

// ProfileIMSServiceProfiles names the IMS service profile of each implicit registration set of a profile's subscribers.
type ProfileIMSServiceProfiles struct {
	MMTel *string `json:"mmtel"`
	MCPTT *string `json:"mcptt"`
}

const (
	imsApplicationServersPath = "api/v1/operator/ims/application-servers"
	imsServiceProfilesPath    = "api/v1/operator/ims/service-profiles"
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

func (c *Client) ListIMSServiceProfiles(ctx context.Context) (*ListIMSServiceProfilesResponse, error) {
	var list ListIMSServiceProfilesResponse

	if err := c.imsRequest(ctx, "GET", imsServiceProfilesPath, nil, &list); err != nil {
		return nil, err
	}

	return &list, nil
}

func (c *Client) GetIMSServiceProfile(ctx context.Context, name string) (*IMSServiceProfile, error) {
	var sp IMSServiceProfile

	if err := c.imsRequest(ctx, "GET", imsServiceProfilesPath+"/"+url.PathEscape(name), nil, &sp); err != nil {
		return nil, err
	}

	return &sp, nil
}

func (c *Client) CreateIMSServiceProfile(ctx context.Context, opts *CreateIMSServiceProfileOptions) (*IMSServiceProfile, error) {
	payload := *opts
	if payload.InitialFilterCriteria == nil {
		payload.InitialFilterCriteria = []IMSInitialFilterCriterion{}
	}

	var sp IMSServiceProfile

	if err := c.imsRequest(ctx, "POST", imsServiceProfilesPath, payload, &sp); err != nil {
		return nil, err
	}

	return &sp, nil
}

func (c *Client) UpdateIMSServiceProfile(ctx context.Context, name string, opts *UpdateIMSServiceProfileOptions) error {
	payload := *opts
	if payload.InitialFilterCriteria == nil {
		payload.InitialFilterCriteria = []IMSInitialFilterCriterion{}
	}

	return c.imsRequest(ctx, "PUT", imsServiceProfilesPath+"/"+url.PathEscape(name), payload, nil)
}

func (c *Client) DeleteIMSServiceProfile(ctx context.Context, name string) error {
	return c.imsRequest(ctx, "DELETE", imsServiceProfilesPath+"/"+url.PathEscape(name), nil, nil)
}

func (c *Client) GetProfileIMSServiceProfiles(ctx context.Context, profileName string) (*ProfileIMSServiceProfiles, error) {
	var assigned ProfileIMSServiceProfiles

	if err := c.imsRequest(ctx, "GET", "api/v1/profiles/"+url.PathEscape(profileName)+"/ims-service-profiles", nil, &assigned); err != nil {
		return nil, err
	}

	return &assigned, nil
}

func (c *Client) SetProfileIMSServiceProfiles(ctx context.Context, profileName string, assigned *ProfileIMSServiceProfiles) error {
	return c.imsRequest(ctx, "PUT", "api/v1/profiles/"+url.PathEscape(profileName)+"/ims-service-profiles", assigned, nil)
}
