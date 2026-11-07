// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/url"
)

type SliceDataNetwork struct {
	DataNetwork     string `json:"data_network"`
	NAT             bool   `json:"nat"`
	LocalSwitch     bool   `json:"local_switch"`
	NetworkInstance string `json:"network_instance"`
}

type ListSliceDataNetworksResponse struct {
	Items []SliceDataNetwork `json:"items"`
}

type CreateSliceDataNetworkOptions struct {
	DataNetwork     string `json:"data_network"`
	NAT             *bool  `json:"nat,omitempty"`
	LocalSwitch     *bool  `json:"local_switch,omitempty"`
	NetworkInstance string `json:"network_instance,omitempty"`
}

type UpdateSliceDataNetworkOptions struct {
	NAT             *bool   `json:"nat,omitempty"`
	LocalSwitch     *bool   `json:"local_switch,omitempty"`
	NetworkInstance *string `json:"network_instance,omitempty"`
}

func sliceDataNetworksPath(slice string) string {
	return "api/v1/slices/" + url.PathEscape(slice) + "/data-networks"
}

func (c *Client) ListSliceDataNetworks(ctx context.Context, slice string) (*ListSliceDataNetworksResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   sliceDataNetworksPath(slice),
	})
	if err != nil {
		return nil, err
	}

	var out ListSliceDataNetworksResponse

	if err := resp.DecodeResult(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *Client) GetSliceDataNetwork(ctx context.Context, slice, dataNetwork string) (*SliceDataNetwork, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   sliceDataNetworksPath(slice) + "/" + url.PathEscape(dataNetwork),
	})
	if err != nil {
		return nil, err
	}

	var out SliceDataNetwork

	if err := resp.DecodeResult(&out); err != nil {
		return nil, err
	}

	return &out, nil
}

func (c *Client) CreateSliceDataNetwork(ctx context.Context, slice string, opts *CreateSliceDataNetworkOptions) error {
	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(opts); err != nil {
		return err
	}

	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   sliceDataNetworksPath(slice),
		Body:   &body,
	})

	return err
}

func (c *Client) UpdateSliceDataNetwork(ctx context.Context, slice, dataNetwork string, opts *UpdateSliceDataNetworkOptions) error {
	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(opts); err != nil {
		return err
	}

	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   sliceDataNetworksPath(slice) + "/" + url.PathEscape(dataNetwork),
		Body:   &body,
	})

	return err
}

func (c *Client) DeleteSliceDataNetwork(ctx context.Context, slice, dataNetwork string) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   sliceDataNetworksPath(slice) + "/" + url.PathEscape(dataNetwork),
	})

	return err
}
