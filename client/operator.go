package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
)

type GetOperatorIDResponse struct {
	Mcc string `json:"mcc,omitempty"`
	Mnc string `json:"mnc,omitempty"`
}

type GetOperatorTrackingResponse struct {
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type HomeNetworkKeyResponse struct {
	ID            int    `json:"id"`
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
	SpnFull  string `json:"spnFull"`
	SpnShort string `json:"spnShort"`
}

type Operator struct {
	ID              GetOperatorIDResponse          `json:"id,omitempty"`
	Tracking        GetOperatorTrackingResponse    `json:"tracking,omitempty"`
	HomeNetworkKeys []HomeNetworkKeyResponse       `json:"homeNetworkKeys,omitempty"`
	NASSecurity     GetOperatorNASSecurityResponse `json:"nasSecurity,omitempty"`
	SPN             GetOperatorSPNResponse         `json:"spn,omitempty"`
}

type UpdateOperatorIDOptions struct {
	Mcc string
	Mnc string
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

// GetOperator retrieves the current operator configuration.
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

// UpdateOperatorID updates the operator's Mobile Country Code (MCC) and Mobile Network Code (MNC).
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

// UpdateOperatorTracking updates the operator's tracking information (supported TACs).
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

// CreateHomeNetworkKey creates a new home network key.
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

// DeleteHomeNetworkKey deletes a home network key by ID.
func (c *Client) DeleteHomeNetworkKey(ctx context.Context, id int) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   fmt.Sprintf("api/v1/operator/home-network-keys/%d", id),
	})
	if err != nil {
		return err
	}

	return nil
}

// GetHomeNetworkKeyPrivateKey retrieves the private key for a home network key by ID.
func (c *Client) GetHomeNetworkKeyPrivateKey(ctx context.Context, id int) (*HomeNetworkKeyPrivateKeyResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   fmt.Sprintf("api/v1/operator/home-network-keys/%d/private-key", id),
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

// UpdateOperatorNASSecurity updates the operator's NAS security algorithm preference order.
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

// UpdateOperatorSPN updates the operator's Service Provider Name (full and short).
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
