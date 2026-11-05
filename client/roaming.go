// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"
)

type RoamingPartnerSEPP struct {
	FQDN           string `json:"fqdn"`
	N32cURI        string `json:"n32cURI"`
	N32fURI        string `json:"n32fURI,omitempty"`
	TrustAnchorPEM string `json:"trustAnchorPEM"`
}

type RoamingPLMN struct {
	Mcc        string `json:"mcc"`
	Mnc        string `json:"mnc"`
	NRFAPIRoot string `json:"nrfAPIRoot,omitempty"`
}

type RoamingPartner struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	PLMNs          []RoamingPLMN       `json:"plmns"`
	ProfileName    string              `json:"profileName"`
	SEPP           *RoamingPartnerSEPP `json:"sepp,omitempty"`
	DiameterPeerID string              `json:"diameterPeerID,omitempty"`
	N32            *RoamingPartnerN32  `json:"n32,omitempty"`
}

type RoamingPartnerN32 struct {
	State     string `json:"state"`
	Peer      string `json:"peer,omitempty"`
	LastError string `json:"lastError,omitempty"`
}

type RoamingPartnerHandshake struct {
	N32 RoamingPartnerN32 `json:"n32"`
}

type SEPPTrustAnchor struct {
	Source      string `json:"source"`
	Subject     string `json:"subject"`
	Fingerprint string `json:"fingerprint"`
	NotAfter    string `json:"notAfter"`
	PEM         string `json:"pem"`
	SEPPIDs     []int  `json:"seppIDs"`
}

type SEPPListener struct {
	SEPPID     int    `json:"seppID,omitempty"`
	Configured bool   `json:"configured"`
	Address    string `json:"address,omitempty"`
	Port       int    `json:"port,omitempty"`
	Listening  bool   `json:"listening"`
	Error      string `json:"error,omitempty"`
}

type SEPPStatus struct {
	Listener     SEPPListener      `json:"listener"`
	Certificates []SEPPCertificate `json:"certificates"`
	TrustAnchors []SEPPTrustAnchor `json:"trustAnchors"`
}

type SEPPCARegenerated struct {
	TrustAnchor SEPPTrustAnchor `json:"trustAnchor"`
	Warnings    []string        `json:"warnings,omitempty"`
}

type ListRoamingPartnersResponse struct {
	Items []RoamingPartner `json:"items"`
}

type RoamingPartnerOptions struct {
	Name           string              `json:"name"`
	PLMNs          []RoamingPLMN       `json:"plmns"`
	ProfileName    string              `json:"profileName"`
	SEPP           *RoamingPartnerSEPP `json:"sepp,omitempty"`
	DiameterPeerID string              `json:"diameterPeerID,omitempty"`
}

type RoamingDiameterPeer struct {
	ID               string `json:"id"`
	DiameterIdentity string `json:"diameterIdentity"`
	Address          string `json:"address"`
	Port             int    `json:"port"`
}

type ListRoamingDiameterPeersResponse struct {
	Items []RoamingDiameterPeer `json:"items"`
}

type RoamingDiameterPeerOptions struct {
	DiameterIdentity string `json:"diameterIdentity"`
	Address          string `json:"address"`
	Port             int    `json:"port,omitempty"`
}

type SEPPCertificate struct {
	SEPPID                 int      `json:"seppID"`
	FQDN                   string   `json:"fqdn"`
	Source                 string   `json:"source"`
	Subject                string   `json:"subject,omitempty"`
	SANs                   []string `json:"sans,omitempty"`
	Issuer                 string   `json:"issuer,omitempty"`
	NotBefore              string   `json:"notBefore,omitempty"`
	NotAfter               string   `json:"notAfter,omitempty"`
	Fingerprint            string   `json:"fingerprint,omitempty"`
	TrustAnchorFingerprint string   `json:"trustAnchorFingerprint,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
}

type ImportSEPPCertificateOptions struct {
	CertificateChainPEM string `json:"certificateChainPEM"`
	PrivateKeyPEM       string `json:"privateKeyPEM"`
	FQDN                string `json:"fqdn,omitempty"`
}

func (c *Client) roamingDo(ctx context.Context, method, path string, payload any, out any) error {
	return c.roamingDoWithin(ctx, method, path, payload, out, 0)
}

func (c *Client) roamingDoWithin(ctx context.Context, method, path string, payload any, out any, timeout time.Duration) error {
	opts := &RequestOptions{
		Type:    SyncRequest,
		Method:  method,
		Path:    path,
		Timeout: timeout,
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

	if out == nil {
		return nil
	}

	return resp.DecodeResult(out)
}

func (c *Client) ListRoamingPartners(ctx context.Context) (*ListRoamingPartnersResponse, error) {
	var partners ListRoamingPartnersResponse

	if err := c.roamingDo(ctx, "GET", "api/v1/operator/roaming/partners", nil, &partners); err != nil {
		return nil, err
	}

	return &partners, nil
}

func (c *Client) GetRoamingPartner(ctx context.Context, id string) (*RoamingPartner, error) {
	var partner RoamingPartner

	if err := c.roamingDo(ctx, "GET", "api/v1/operator/roaming/partners/"+url.PathEscape(id), nil, &partner); err != nil {
		return nil, err
	}

	return &partner, nil
}

func (c *Client) CreateRoamingPartner(ctx context.Context, opts *RoamingPartnerOptions) (*RoamingPartner, error) {
	var partner RoamingPartner

	if err := c.roamingDo(ctx, "POST", "api/v1/operator/roaming/partners", opts, &partner); err != nil {
		return nil, err
	}

	return &partner, nil
}

func (c *Client) UpdateRoamingPartner(ctx context.Context, id string, opts *RoamingPartnerOptions) error {
	return c.roamingDoWithin(ctx, "PUT", "api/v1/operator/roaming/partners/"+url.PathEscape(id), opts, nil, withdrawalTimeout)
}

func (c *Client) DeleteRoamingPartner(ctx context.Context, id string) error {
	return c.roamingDoWithin(ctx, "DELETE", "api/v1/operator/roaming/partners/"+url.PathEscape(id), nil, nil, withdrawalTimeout)
}

func (c *Client) RehandshakeRoamingPartner(ctx context.Context, id string) (*RoamingPartnerHandshake, error) {
	var st RoamingPartnerHandshake

	if err := c.roamingDo(ctx, "POST", "api/v1/operator/roaming/partners/"+url.PathEscape(id)+"/handshake", nil, &st); err != nil {
		return nil, err
	}

	return &st, nil
}

func (c *Client) GetSEPPStatus(ctx context.Context) (*SEPPStatus, error) {
	var st SEPPStatus

	if err := c.roamingDo(ctx, "GET", "api/v1/operator/roaming/sepp", nil, &st); err != nil {
		return nil, err
	}

	return &st, nil
}

func (c *Client) ListRoamingDiameterPeers(ctx context.Context) (*ListRoamingDiameterPeersResponse, error) {
	var peers ListRoamingDiameterPeersResponse

	if err := c.roamingDo(ctx, "GET", "api/v1/operator/roaming/diameter-peers", nil, &peers); err != nil {
		return nil, err
	}

	return &peers, nil
}

func (c *Client) GetRoamingDiameterPeer(ctx context.Context, id string) (*RoamingDiameterPeer, error) {
	var peer RoamingDiameterPeer

	if err := c.roamingDo(ctx, "GET", "api/v1/operator/roaming/diameter-peers/"+url.PathEscape(id), nil, &peer); err != nil {
		return nil, err
	}

	return &peer, nil
}

func (c *Client) CreateRoamingDiameterPeer(ctx context.Context, opts *RoamingDiameterPeerOptions) (*RoamingDiameterPeer, error) {
	var peer RoamingDiameterPeer

	if err := c.roamingDo(ctx, "POST", "api/v1/operator/roaming/diameter-peers", opts, &peer); err != nil {
		return nil, err
	}

	return &peer, nil
}

func (c *Client) UpdateRoamingDiameterPeer(ctx context.Context, id string, opts *RoamingDiameterPeerOptions) error {
	return c.roamingDo(ctx, "PUT", "api/v1/operator/roaming/diameter-peers/"+url.PathEscape(id), opts, nil)
}

func (c *Client) DeleteRoamingDiameterPeer(ctx context.Context, id string) error {
	return c.roamingDo(ctx, "DELETE", "api/v1/operator/roaming/diameter-peers/"+url.PathEscape(id), nil, nil)
}

func seppCertificatePath(seppID int) string {
	return "api/v1/operator/roaming/sepp/nodes/" + strconv.Itoa(seppID) + "/certificate"
}

func (c *Client) ImportSEPPCertificate(ctx context.Context, seppID int, opts *ImportSEPPCertificateOptions) (*SEPPCertificate, error) {
	var cert SEPPCertificate

	if err := c.roamingDo(ctx, "PUT", seppCertificatePath(seppID), opts, &cert); err != nil {
		return nil, err
	}

	return &cert, nil
}

func (c *Client) DeleteSEPPCertificate(ctx context.Context, seppID int) error {
	return c.roamingDo(ctx, "DELETE", seppCertificatePath(seppID), nil, nil)
}

func (c *Client) RegenerateSEPPCA(ctx context.Context) (*SEPPCARegenerated, error) {
	var out SEPPCARegenerated

	if err := c.roamingDo(ctx, "POST", "api/v1/operator/roaming/sepp/ca/regenerate", nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}
