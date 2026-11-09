// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
	"net/url"
)

const allowedIMSPeersPath = "api/v1/integrations/ims/allowed-peers"

type IMSSettings struct {
	PCSCFAddresses    []string `json:"pcscfAddresses"`
	VoiceStatus       string   `json:"voiceStatus"`
	HomeDomain        string   `json:"homeDomain"`
	DerivedHomeDomain string   `json:"derivedHomeDomain"`
	AdvertiseVoice    bool     `json:"advertiseVoice"`
	RestrictPeers     bool     `json:"restrictPeers"`
}

type UpdateIMSSettingsOptions struct {
	PCSCFAddresses []string
	HomeDomain     string
	AdvertiseVoice bool
	RestrictPeers  bool
}

type AllowedIMSPeer struct {
	Name        string               `json:"name"`
	OriginHost  string               `json:"originHost,omitempty"`
	Realm       string               `json:"realm,omitempty"`
	Addresses   []string             `json:"addresses"`
	Connections []DiameterPeerStatus `json:"connections"`
}

type ListAllowedIMSPeersResponse struct {
	Items []AllowedIMSPeer `json:"items"`
}

type CreateAllowedIMSPeerOptions struct {
	Name       string   `json:"name"`
	OriginHost string   `json:"originHost"`
	Realm      string   `json:"realm"`
	Addresses  []string `json:"addresses"`
}

type UpdateAllowedIMSPeerOptions struct {
	OriginHost string   `json:"originHost"`
	Realm      string   `json:"realm"`
	Addresses  []string `json:"addresses"`
}

func (c *Client) GetIMSSettings(ctx context.Context) (*IMSSettings, error) {
	var settings IMSSettings

	if err := c.integrationsRequest(ctx, "GET", "api/v1/integrations/ims", nil, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func (c *Client) UpdateIMSSettings(ctx context.Context, opts *UpdateIMSSettingsOptions) error {
	payload := struct {
		PCSCFAddresses []string `json:"pcscfAddresses"`
		HomeDomain     string   `json:"homeDomain"`
		AdvertiseVoice bool     `json:"advertiseVoice"`
		RestrictPeers  bool     `json:"restrictPeers"`
	}{
		PCSCFAddresses: opts.PCSCFAddresses,
		HomeDomain:     opts.HomeDomain,
		AdvertiseVoice: opts.AdvertiseVoice,
		RestrictPeers:  opts.RestrictPeers,
	}

	if payload.PCSCFAddresses == nil {
		payload.PCSCFAddresses = []string{}
	}

	return c.integrationsRequest(ctx, "PUT", "api/v1/integrations/ims", payload, nil)
}

func (c *Client) ListAllowedIMSPeers(ctx context.Context) (*ListAllowedIMSPeersResponse, error) {
	var peers ListAllowedIMSPeersResponse

	if err := c.integrationsRequest(ctx, "GET", allowedIMSPeersPath, nil, &peers); err != nil {
		return nil, err
	}

	return &peers, nil
}

func (c *Client) GetAllowedIMSPeer(ctx context.Context, name string) (*AllowedIMSPeer, error) {
	var peer AllowedIMSPeer

	if err := c.integrationsRequest(ctx, "GET", allowedIMSPeersPath+"/"+url.PathEscape(name), nil, &peer); err != nil {
		return nil, err
	}

	return &peer, nil
}

func (c *Client) CreateAllowedIMSPeer(ctx context.Context, opts *CreateAllowedIMSPeerOptions) (*AllowedIMSPeer, error) {
	var peer AllowedIMSPeer

	if err := c.integrationsRequest(ctx, "POST", allowedIMSPeersPath, opts, &peer); err != nil {
		return nil, err
	}

	return &peer, nil
}

func (c *Client) UpdateAllowedIMSPeer(ctx context.Context, name string, opts *UpdateAllowedIMSPeerOptions) error {
	return c.integrationsRequest(ctx, "PUT", allowedIMSPeersPath+"/"+url.PathEscape(name), opts, nil)
}

func (c *Client) DeleteAllowedIMSPeer(ctx context.Context, name string) error {
	return c.integrationsRequest(ctx, "DELETE", allowedIMSPeersPath+"/"+url.PathEscape(name), nil, nil)
}
