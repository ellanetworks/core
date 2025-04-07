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
	Sst int `json:"sst,omitempty"`
	Sd  int `json:"sd,omitempty"`
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
	Sd  int
}

type UpdateOperatorTrackingOptions struct {
	SupportedTacs []string
}

func (c *Client) GetOperator() (*Operator, error) {
	resp, err := c.Requester.Do(context.Background(), &RequestOptions{
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

func (c *Client) UpdateOperatorID(opts *UpdateOperatorIDOptions) error {
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

	_, err = c.Requester.Do(context.Background(), &RequestOptions{
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

func (c *Client) UpdateOperatorSlice(opts *UpdateOperatorSliceOptions) error {
	payload := struct {
		Sst int `json:"sst"`
		Sd  int `json:"sd"`
	}{
		Sst: opts.Sst,
		Sd:  opts.Sd,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(context.Background(), &RequestOptions{
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

func (c *Client) UpdateOperatorTracking(opts *UpdateOperatorTrackingOptions) error {
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

	_, err = c.Requester.Do(context.Background(), &RequestOptions{
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
