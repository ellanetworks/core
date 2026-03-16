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

type GetOperatorSliceResponse struct {
	Sst int    `json:"sst,omitempty"`
	Sd  string `json:"sd,omitempty"`
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

type GetOperatorHomeNetworkResponse struct {
	Keys []HomeNetworkKeyResponse `json:"keys"`
}

type CreateHomeNetworkKeyOptions struct {
	KeyIdentifier int
	Scheme        string
	PrivateKey    string
}

type GetOperatorSecurityResponse struct {
	CipheringOrder []string `json:"cipheringOrder,omitempty"`
	IntegrityOrder []string `json:"integrityOrder,omitempty"`
}

type Operator struct {
	ID          GetOperatorIDResponse          `json:"id,omitempty"`
	Slice       GetOperatorSliceResponse       `json:"slice,omitempty"`
	Tracking    GetOperatorTrackingResponse    `json:"tracking,omitempty"`
	HomeNetwork GetOperatorHomeNetworkResponse `json:"homeNetwork,omitempty"`
	Security    GetOperatorSecurityResponse    `json:"security,omitempty"`
}

type UpdateOperatorIDOptions struct {
	Mcc string
	Mnc string
}

type UpdateOperatorSliceOptions struct {
	Sst int
	Sd  string
}

type UpdateOperatorTrackingOptions struct {
	SupportedTacs []string
}

type UpdateOperatorSecurityOptions struct {
	CipheringOrder []string
	IntegrityOrder []string
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

// UpdateOperatorSlice updates the operator's slice information.
func (c *Client) UpdateOperatorSlice(ctx context.Context, opts *UpdateOperatorSliceOptions) error {
	payload := struct {
		Sst int    `json:"sst"`
		Sd  string `json:"sd"`
	}{
		Sst: opts.Sst,
		Sd:  opts.Sd,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/operator/slice",
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

// ListHomeNetworkKeys retrieves all home network keys.
func (c *Client) ListHomeNetworkKeys(ctx context.Context) ([]HomeNetworkKeyResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/operator/home-network-keys",
	})
	if err != nil {
		return nil, err
	}

	var keys []HomeNetworkKeyResponse

	err = resp.DecodeResult(&keys)
	if err != nil {
		return nil, err
	}

	return keys, nil
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

// UpdateOperatorSecurity updates the operator's NAS security algorithm preference order.
func (c *Client) UpdateOperatorSecurity(ctx context.Context, opts *UpdateOperatorSecurityOptions) error {
	payload := struct {
		CipheringOrder []string `json:"cipheringOrder"`
		IntegrityOrder []string `json:"integrityOrder"`
	}{
		CipheringOrder: opts.CipheringOrder,
		IntegrityOrder: opts.IntegrityOrder,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/operator/security",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}
