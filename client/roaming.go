// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

type RoamingPartnerSEPP struct {
	FQDN           string `json:"fqdn"`
	N32cURI        string `json:"n32cURI"`
	N32fURI        string `json:"n32fURI,omitempty"`
	TrustAnchorPEM string `json:"trustAnchorPEM"`
	NRFAPIRoot     string `json:"nrfAPIRoot,omitempty"`
}

type RoamingPLMN struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type RoamingPartner struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	PLMNs          []RoamingPLMN       `json:"plmns"`
	ProfileName    string              `json:"profileName"`
	SEPP           *RoamingPartnerSEPP `json:"sepp,omitempty"`
	DiameterPeerID string              `json:"diameterPeerID,omitempty"`
	N32            []RoamingPartnerN32 `json:"n32,omitempty"`
}

type RoamingPartnerN32 struct {
	NodeID     string                  `json:"nodeID,omitempty"`
	SEPPID     int                     `json:"seppID,omitempty"`
	State      string                  `json:"state"`
	Since      string                  `json:"since,omitempty"`
	Initiator  string                  `json:"initiator,omitempty"`
	NFTLST     bool                    `json:"nftlst"`
	Peer       string                  `json:"peer,omitempty"`
	LastError  string                  `json:"lastError,omitempty"`
	OtherPeers []RoamingPartnerN32Peer `json:"otherPeers,omitempty"`
}

type RoamingPartnerN32Peer struct {
	Peer   string `json:"peer"`
	Since  string `json:"since"`
	NFTLST bool   `json:"nftlst"`
}

type RoamingPartnerHandshake struct {
	N32 []RoamingPartnerN32 `json:"n32"`
}

type SEPPCertificateSummary struct {
	Subject                string   `json:"subject"`
	SANs                   []string `json:"sans"`
	Issuer                 string   `json:"issuer"`
	NotBefore              string   `json:"notBefore"`
	NotAfter               string   `json:"notAfter"`
	Fingerprint            string   `json:"fingerprint"`
	TrustAnchorFingerprint string   `json:"trustAnchorFingerprint"`
}

type SEPPTrustAnchor struct {
	Source      string `json:"source"`
	Subject     string `json:"subject"`
	Fingerprint string `json:"fingerprint"`
	NotAfter    string `json:"notAfter"`
	PEM         string `json:"pem"`
	SEPPIDs     []int  `json:"seppIDs"`
}

type SEPPNodeStatus struct {
	NodeID        string                  `json:"nodeID,omitempty"`
	SEPPID        int                     `json:"seppID,omitempty"`
	FQDN          string                  `json:"fqdn,omitempty"`
	Source        string                  `json:"source,omitempty"`
	Reachable     bool                    `json:"reachable"`
	N32Configured bool                    `json:"n32Configured"`
	Address       string                  `json:"address,omitempty"`
	Port          int                     `json:"port,omitempty"`
	Listening     bool                    `json:"listening"`
	Certificate   *SEPPCertificateSummary `json:"certificate,omitempty"`
	Error         string                  `json:"error,omitempty"`
}

type SEPPStatus struct {
	TrustAnchors []SEPPTrustAnchor `json:"trustAnchors"`
	Nodes        []SEPPNodeStatus  `json:"nodes"`
}

type SEPPCARegenerated struct {
	TrustAnchor SEPPTrustAnchor `json:"trustAnchor"`
	Reissued    []int           `json:"reissued"`
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
	SEPPID              int      `json:"seppID"`
	Source              string   `json:"source"`
	FQDN                string   `json:"fqdn"`
	CertificateChainPEM string   `json:"certificateChainPEM"`
	TrustAnchorPEM      string   `json:"trustAnchorPEM"`
	Fingerprint         string   `json:"fingerprint"`
	NotAfter            string   `json:"notAfter"`
	Warnings            []string `json:"warnings,omitempty"`
}

type ImportSEPPCertificateOptions struct {
	CertificateChainPEM string `json:"certificateChainPEM"`
	PrivateKeyPEM       string `json:"privateKeyPEM"`
	FQDN                string `json:"fqdn,omitempty"`
}

type RoamingUsage struct {
	Date          string `json:"date"`
	Mcc           string `json:"mcc"`
	Mnc           string `json:"mnc"`
	PartnerName   string `json:"partner_name,omitempty"`
	UplinkBytes   int64  `json:"uplink_bytes"`
	DownlinkBytes int64  `json:"downlink_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
}

type ListRoamingUsageParams struct {
	Start string
	End   string
	IMSI  string
}

func (c *Client) roamingDo(ctx context.Context, method, path string, query url.Values, payload any, out any) error {
	opts := &RequestOptions{
		Type:   SyncRequest,
		Method: method,
		Path:   path,
		Query:  query,
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

	if err := c.roamingDo(ctx, "GET", "api/v1/roaming/partners", nil, nil, &partners); err != nil {
		return nil, err
	}

	return &partners, nil
}

func (c *Client) GetRoamingPartner(ctx context.Context, id string) (*RoamingPartner, error) {
	var partner RoamingPartner

	if err := c.roamingDo(ctx, "GET", "api/v1/roaming/partners/"+id, nil, nil, &partner); err != nil {
		return nil, err
	}

	return &partner, nil
}

func (c *Client) CreateRoamingPartner(ctx context.Context, opts *RoamingPartnerOptions) (*RoamingPartner, error) {
	var partner RoamingPartner

	if err := c.roamingDo(ctx, "POST", "api/v1/roaming/partners", nil, opts, &partner); err != nil {
		return nil, err
	}

	return &partner, nil
}

func (c *Client) UpdateRoamingPartner(ctx context.Context, id string, opts *RoamingPartnerOptions) error {
	return c.roamingDo(ctx, "PUT", "api/v1/roaming/partners/"+id, nil, opts, nil)
}

func (c *Client) DeleteRoamingPartner(ctx context.Context, id string) error {
	return c.roamingDo(ctx, "DELETE", "api/v1/roaming/partners/"+id, nil, nil, nil)
}

func (c *Client) RehandshakeRoamingPartner(ctx context.Context, id string) (*RoamingPartnerHandshake, error) {
	var st RoamingPartnerHandshake

	if err := c.roamingDo(ctx, "POST", "api/v1/roaming/partners/"+id+"/handshake", nil, nil, &st); err != nil {
		return nil, err
	}

	return &st, nil
}

func (c *Client) GetSEPPStatus(ctx context.Context) (*SEPPStatus, error) {
	var st SEPPStatus

	if err := c.roamingDo(ctx, "GET", "api/v1/roaming/sepp", nil, nil, &st); err != nil {
		return nil, err
	}

	return &st, nil
}

func (c *Client) ListRoamingDiameterPeers(ctx context.Context) (*ListRoamingDiameterPeersResponse, error) {
	var peers ListRoamingDiameterPeersResponse

	if err := c.roamingDo(ctx, "GET", "api/v1/roaming/diameter-peers", nil, nil, &peers); err != nil {
		return nil, err
	}

	return &peers, nil
}

func (c *Client) GetRoamingDiameterPeer(ctx context.Context, id string) (*RoamingDiameterPeer, error) {
	var peer RoamingDiameterPeer

	if err := c.roamingDo(ctx, "GET", "api/v1/roaming/diameter-peers/"+id, nil, nil, &peer); err != nil {
		return nil, err
	}

	return &peer, nil
}

func (c *Client) CreateRoamingDiameterPeer(ctx context.Context, opts *RoamingDiameterPeerOptions) (*RoamingDiameterPeer, error) {
	var peer RoamingDiameterPeer

	if err := c.roamingDo(ctx, "POST", "api/v1/roaming/diameter-peers", nil, opts, &peer); err != nil {
		return nil, err
	}

	return &peer, nil
}

func (c *Client) UpdateRoamingDiameterPeer(ctx context.Context, id string, opts *RoamingDiameterPeerOptions) error {
	return c.roamingDo(ctx, "PUT", "api/v1/roaming/diameter-peers/"+id, nil, opts, nil)
}

func (c *Client) DeleteRoamingDiameterPeer(ctx context.Context, id string) error {
	return c.roamingDo(ctx, "DELETE", "api/v1/roaming/diameter-peers/"+id, nil, nil, nil)
}

func seppCertificatePath(seppID int) string {
	return "api/v1/roaming/sepp/nodes/" + strconv.Itoa(seppID) + "/certificate"
}

func (c *Client) GetSEPPCertificate(ctx context.Context, seppID int) (*SEPPCertificate, error) {
	var cert SEPPCertificate

	if err := c.roamingDo(ctx, "GET", seppCertificatePath(seppID), nil, nil, &cert); err != nil {
		return nil, err
	}

	return &cert, nil
}

func (c *Client) ImportSEPPCertificate(ctx context.Context, seppID int, opts *ImportSEPPCertificateOptions) (*SEPPCertificate, error) {
	var cert SEPPCertificate

	if err := c.roamingDo(ctx, "PUT", seppCertificatePath(seppID), nil, opts, &cert); err != nil {
		return nil, err
	}

	return &cert, nil
}

func (c *Client) GenerateSEPPCertificate(ctx context.Context, seppID int) (*SEPPCertificate, error) {
	var cert SEPPCertificate

	if err := c.roamingDo(ctx, "POST", seppCertificatePath(seppID)+"/generate", nil, nil, &cert); err != nil {
		return nil, err
	}

	return &cert, nil
}

func (c *Client) RegenerateSEPPCA(ctx context.Context) (*SEPPCARegenerated, error) {
	var out SEPPCARegenerated

	if err := c.roamingDo(ctx, "POST", "api/v1/roaming/sepp/ca/regenerate", nil, nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *Client) ListRoamingUsage(ctx context.Context, p *ListRoamingUsageParams) ([]RoamingUsage, error) {
	query := url.Values{}

	if p != nil {
		for key, value := range map[string]string{"start": p.Start, "end": p.End, "imsi": p.IMSI} {
			if value != "" {
				query.Set(key, value)
			}
		}
	}

	var usage []RoamingUsage

	if err := c.roamingDo(ctx, "GET", "api/v1/roaming/usage", query, nil, &usage); err != nil {
		return nil, err
	}

	return usage, nil
}
