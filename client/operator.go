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

type GetOperatorSliceResponse struct {
	Sst int    `json:"sst,omitempty"`
	Sd  string `json:"sd,omitempty"`
}

type GetOperatorTrackingResponse struct {
	SupportedTacs []string `json:"supportedTacs,omitempty"`
}

type GetOperatorHomeNetworkResponse struct {
	PublicKey string `json:"publicKey,omitempty"`
}

type Operator struct {
	ID          GetOperatorIDResponse          `json:"id,omitempty"`
	Slice       GetOperatorSliceResponse       `json:"slice,omitempty"`
	Tracking    GetOperatorTrackingResponse    `json:"tracking,omitempty"`
	HomeNetwork GetOperatorHomeNetworkResponse `json:"homeNetwork,omitempty"`
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

type UpdateOperatorHomeNetworkOptions struct {
	PrivateKey string
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

func (c *Client) UpdateOperatorHomeNetwork(ctx context.Context, opts *UpdateOperatorHomeNetworkOptions) error {
	payload := struct {
		PrivateKey string `json:"privateKey"`
	}{
		PrivateKey: opts.PrivateKey,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/operator/home-network",
		Body:   &body,
	})
	if err != nil {
		return err
	}
	return nil
}
