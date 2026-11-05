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

type SMSC struct {
	Name           string     `json:"name"`
	ServiceCentres []string   `json:"serviceCentres"`
	Nodes          []SMSCNode `json:"nodes"`
}

type SMSCNode struct {
	DiameterIdentity string          `json:"diameterIdentity"`
	Address          string          `json:"address"`
	Port             int             `json:"port"`
	Status           *SMSCNodeStatus `json:"status,omitempty"`
}

type SMSCNodeStatus struct {
	State string `json:"state"`
	Host  string `json:"host,omitempty"`
	Realm string `json:"realm,omitempty"`
	Since string `json:"since"`
	Error string `json:"error,omitempty"`
}

type ListSMSCsResponse struct {
	Items []SMSC `json:"items"`
}

type GetOperatorIMSResponse struct {
	PCSCFAddresses []string `json:"pcscfAddresses"`
	VoiceStatus    string   `json:"voiceStatus"`
	HomeDomain     string   `json:"homeDomain"`
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

type SMSCNodeOptions struct {
	DiameterIdentity string `json:"diameterIdentity"`
	Address          string `json:"address"`
	Port             int    `json:"port,omitempty"`
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

func (c *Client) ListSMSCs(ctx context.Context) (*ListSMSCsResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/operator/sms/smscs",
	})
	if err != nil {
		return nil, err
	}

	var smscs ListSMSCsResponse

	if err := resp.DecodeResult(&smscs); err != nil {
		return nil, err
	}

	return &smscs, nil
}

func (c *Client) GetSMSC(ctx context.Context, name string) (*SMSC, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/operator/sms/smscs/" + name,
	})
	if err != nil {
		return nil, err
	}

	var smsc SMSC

	if err := resp.DecodeResult(&smsc); err != nil {
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

	resp, err := c.writeSMSC(ctx, "POST", "api/v1/operator/sms/smscs", payload)
	if err != nil {
		return nil, err
	}

	var smsc SMSC

	if err := resp.DecodeResult(&smsc); err != nil {
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

	_, err := c.writeSMSC(ctx, "PUT", "api/v1/operator/sms/smscs/"+name, payload)

	return err
}

func (c *Client) writeSMSC(ctx context.Context, method, path string, payload any) (*RequestResponse, error) {
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

func (c *Client) DeleteSMSC(ctx context.Context, name string) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/operator/sms/smscs/" + name,
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
