package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type CreateDataNetworkOptions struct {
	Name   string `json:"name"`
	IPPool string `json:"ip-pool"`
	DNS    string `json:"dns"`
	Mtu    int32  `json:"mtu"`
}

type GetDataNetworkOptions struct {
	Name string `json:"name"`
}

type DeleteDataNetworkOptions struct {
	Name string `json:"name"`
}

type DataNetwork struct {
	Name   string `json:"name"`
	IPPool string `json:"ip-pool"`
	DNS    string `json:"dns"`
	Mtu    int32  `json:"mtu"`
}

func (c *Client) CreateDataNetwork(ctx context.Context, opts *CreateDataNetworkOptions) error {
	payload := struct {
		Name   string `json:"name"`
		IPPool string `json:"ip-pool"`
		DNS    string `json:"dns"`
		Mtu    int32  `json:"mtu"`
	}{
		Name:   opts.Name,
		IPPool: opts.IPPool,
		DNS:    opts.DNS,
		Mtu:    opts.Mtu,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/data-networks",
		Body:   &body,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetDataNetwork(ctx context.Context, opts *GetDataNetworkOptions) (*DataNetwork, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/data-networks/" + opts.Name,
	})
	if err != nil {
		return nil, err
	}

	var dataNetworkResponse DataNetwork

	err = resp.DecodeResult(&dataNetworkResponse)
	if err != nil {
		return nil, err
	}
	return &dataNetworkResponse, nil
}

func (c *Client) DeleteDataNetwork(ctx context.Context, opts *DeleteDataNetworkOptions) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/data-networks/" + opts.Name,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ListDataNetworks(ctx context.Context) ([]*DataNetwork, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/data-networks",
	})
	if err != nil {
		return nil, err
	}
	var dataNetworks []*DataNetwork
	err = resp.DecodeResult(&dataNetworks)
	if err != nil {
		return nil, err
	}
	return dataNetworks, nil
}
