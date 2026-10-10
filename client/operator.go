// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type GetOperatorIDResponse struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type GetOperatorTrackingResponse struct {
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type HomeNetworkKeyResponse struct {
	ID            string `json:"id"` // UUIDv7
	KeyIdentifier int    `json:"keyIdentifier"`
	Scheme        string `json:"scheme"`
	PublicKey     string `json:"publicKey"`
}

type HomeNetworkKeyPrivateKeyResponse struct {
	PrivateKey string `json:"privateKey"`
}

type GetOperatorNASSecurityResponse struct {
	Ciphering []string `json:"ciphering,omitempty"`
	Integrity []string `json:"integrity,omitempty"`
}

type CreateHomeNetworkKeyOptions struct {
	KeyIdentifier int
	Scheme        string
	PrivateKey    string
}

type GetOperatorSPNResponse struct {
	FullName  string `json:"fullName"`
	ShortName string `json:"shortName"`
}

type GetOperatorSMSResponse struct {
	SMSNumber string `json:"smsNumber"`
}

type SMSCPeer struct {
	ID               string          `json:"id"`
	DiameterIdentity string          `json:"diameterIdentity"`
	Address          string          `json:"address"`
	Port             int             `json:"port"`
	ServiceCentres   []string        `json:"serviceCentres"`
	Status           *SMSCPeerStatus `json:"status,omitempty"`
}

type SMSCPeerStatus struct {
	State string `json:"state"`
	Host  string `json:"host,omitempty"`
	Realm string `json:"realm,omitempty"`
	Since string `json:"since"`
	Error string `json:"error,omitempty"`
}

type ListSMSCPeersResponse struct {
	Items []SMSCPeer `json:"items"`
}

type GetOperatorIMSResponse struct {
	PCSCFAddresses []string `json:"pcscfAddresses"`
}

type Operator struct {
	ID              GetOperatorIDResponse          `json:"id,omitempty"`
	Tracking        GetOperatorTrackingResponse    `json:"tracking,omitempty"`
	HomeNetworkKeys []HomeNetworkKeyResponse       `json:"homeNetworkKeys,omitempty"`
	NASSecurity     GetOperatorNASSecurityResponse `json:"nasSecurity,omitempty"`
	SPN             GetOperatorSPNResponse         `json:"spn,omitempty"`
	SMS             GetOperatorSMSResponse         `json:"sms,omitempty"`
	IMS             GetOperatorIMSResponse         `json:"ims,omitempty"`
}

type UpdateOperatorIDOptions struct {
	Mcc string
	Mnc string
}

// UpdateOperatorCodeOptions sets the operator code (OPC root used by
// MILENAGE). Must be a 32-character hex string. The server rejects the
// update when any subscribers exist.
type UpdateOperatorCodeOptions struct {
	OperatorCode string
}

type UpdateOperatorTrackingOptions struct {
	SupportedTacs []string
}

type UpdateOperatorNASSecurityOptions struct {
	Ciphering []string
	Integrity []string
}

type UpdateOperatorSPNOptions struct {
	FullName  string
	ShortName string
}

type UpdateOperatorSMSOptions struct {
	SMSNumber string
}

type SMSCPeerOptions struct {
	DiameterIdentity string
	Address          string
	Port             int
	ServiceCentres   []string
}

type UpdateOperatorIMSOptions struct {
	PCSCFAddresses []string
}

func (c *Client) GetOperator(ctx context.Context) (*Operator, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/operator",
	})
	if err != nil {
		return nil, err
	}

	var operatorResponse Operator

	err = resp.DecodeResult(&operatorResponse)
	if err != nil {
		return nil, err
	}

	return &operatorResponse, nil
}

func (c *Client) UpdateOperatorID(ctx context.Context, opts *UpdateOperatorIDOptions) error {
	payload := struct {
		Mcc string `json:"mcc"`
		Mnc string `json:"mnc"`
	}{
		Mcc: opts.Mcc,
		Mnc: opts.Mnc,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/operator/id",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) UpdateOperatorCode(ctx context.Context, opts *UpdateOperatorCodeOptions) error {
	payload := struct {
		OperatorCode string `json:"operatorCode,omitempty"`
	}{
		OperatorCode: opts.OperatorCode,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/operator/code",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) UpdateOperatorTracking(ctx context.Context, opts *UpdateOperatorTrackingOptions) error {
	payload := struct {
		SupportedTacs []string `json:"supportedTacs"`
	}{
		SupportedTacs: opts.SupportedTacs,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/operator/tracking",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) CreateHomeNetworkKey(ctx context.Context, opts *CreateHomeNetworkKeyOptions) error {
	payload := struct {
		KeyIdentifier int    `json:"keyIdentifier"`
		Scheme        string `json:"scheme"`
		PrivateKey    string `json:"privateKey"`
	}{
		KeyIdentifier: opts.KeyIdentifier,
		Scheme:        opts.Scheme,
		PrivateKey:    opts.PrivateKey,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/operator/home-network-keys",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// DeleteHomeNetworkKey deletes a home network key by ID. The ID is the
// UUIDv7 string returned in Operator.HomeNetworkKeys.
func (c *Client) DeleteHomeNetworkKey(ctx context.Context, id string) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/operator/home-network-keys/" + id,
	})
	if err != nil {
		return err
	}

	return nil
}

// GetHomeNetworkKeyPrivateKey returns the private key for a home network
// key. The ID is the UUIDv7 string returned in Operator.HomeNetworkKeys.
func (c *Client) GetHomeNetworkKeyPrivateKey(ctx context.Context, id string) (*HomeNetworkKeyPrivateKeyResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/operator/home-network-keys/" + id + "/private-key",
	})
	if err != nil {
		return nil, err
	}

	var keyResponse HomeNetworkKeyPrivateKeyResponse

	err = resp.DecodeResult(&keyResponse)
	if err != nil {
		return nil, err
	}

	return &keyResponse, nil
}

func (c *Client) UpdateOperatorNASSecurity(ctx context.Context, opts *UpdateOperatorNASSecurityOptions) error {
	payload := struct {
		Ciphering []string `json:"ciphering"`
		Integrity []string `json:"integrity"`
	}{
		Ciphering: opts.Ciphering,
		Integrity: opts.Integrity,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/operator/nas-security",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) UpdateOperatorSPN(ctx context.Context, opts *UpdateOperatorSPNOptions) error {
	payload := struct {
		FullName  string `json:"fullName"`
		ShortName string `json:"shortName"`
	}{
		FullName:  opts.FullName,
		ShortName: opts.ShortName,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/operator/spn",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) UpdateOperatorSMS(ctx context.Context, opts *UpdateOperatorSMSOptions) error {
	payload := struct {
		SMSNumber string `json:"smsNumber"`
	}{
		SMSNumber: opts.SMSNumber,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/operator/sms",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) ListSMSCPeers(ctx context.Context) (*ListSMSCPeersResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/operator/sms/smsc-peers",
	})
	if err != nil {
		return nil, err
	}

	var peers ListSMSCPeersResponse

	if err := resp.DecodeResult(&peers); err != nil {
		return nil, err
	}

	return &peers, nil
}

func (c *Client) GetSMSCPeer(ctx context.Context, id string) (*SMSCPeer, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/operator/sms/smsc-peers/" + id,
	})
	if err != nil {
		return nil, err
	}

	var peer SMSCPeer

	if err := resp.DecodeResult(&peer); err != nil {
		return nil, err
	}

	return &peer, nil
}

func (c *Client) CreateSMSCPeer(ctx context.Context, opts *SMSCPeerOptions) (*SMSCPeer, error) {
	resp, err := c.writeSMSCPeer(ctx, "POST", "api/v1/operator/sms/smsc-peers", opts)
	if err != nil {
		return nil, err
	}

	var peer SMSCPeer

	if err := resp.DecodeResult(&peer); err != nil {
		return nil, err
	}

	return &peer, nil
}

func (c *Client) UpdateSMSCPeer(ctx context.Context, id string, opts *SMSCPeerOptions) error {
	_, err := c.writeSMSCPeer(ctx, "PUT", "api/v1/operator/sms/smsc-peers/"+id, opts)

	return err
}

func (c *Client) writeSMSCPeer(ctx context.Context, method, path string, opts *SMSCPeerOptions) (*RequestResponse, error) {
	payload := struct {
		DiameterIdentity string   `json:"diameterIdentity"`
		Address          string   `json:"address"`
		Port             int      `json:"port,omitempty"`
		ServiceCentres   []string `json:"serviceCentres"`
	}{
		DiameterIdentity: opts.DiameterIdentity,
		Address:          opts.Address,
		Port:             opts.Port,
		ServiceCentres:   opts.ServiceCentres,
	}

	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		return nil, err
	}

	return c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: method,
		Path:   path,
		Body:   &body,
	})
}

func (c *Client) DeleteSMSCPeer(ctx context.Context, id string) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/operator/sms/smsc-peers/" + id,
	})

	return err
}

func (c *Client) UpdateOperatorIMS(ctx context.Context, opts *UpdateOperatorIMSOptions) error {
	payload := struct {
		PCSCFAddresses []string `json:"pcscfAddresses"`
	}{
		PCSCFAddresses: opts.PCSCFAddresses,
	}

	if payload.PCSCFAddresses == nil {
		payload.PCSCFAddresses = []string{}
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/operator/ims",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}
