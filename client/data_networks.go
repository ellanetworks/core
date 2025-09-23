package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type CreateDataNetworkOptions struct {
	Name   string `json:"name"`
	IPPool string `json:"ip_pool"`
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
	IPPool string `json:"ip_pool"`
	DNS    string `json:"dns"`
	Mtu    int32  `json:"mtu"`
}

type ListDataNetworksResponse struct {
	Items      []DataNetwork `json:"items"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalCount int           `json:"total_count"`
}

func (c *Client) CreateDataNetwork(ctx context.Context, opts *CreateDataNetworkOptions) error {
	payload := struct {
		Name   string `json:"name"`
		IPPool string `json:"ip_pool"`
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
		Path:   "api/v1/networking/data-networks",
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
		Path:   "api/v1/networking/data-networks/" + opts.Name,
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
		Path:   "api/v1/networking/data-networks/" + opts.Name,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ListDataNetworks(ctx context.Context, p *ListParams) (*ListDataNetworksResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/data-networks",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var dataNetworks ListDataNetworksResponse

	err = resp.DecodeResult(&dataNetworks)
	if err != nil {
		return nil, err
	}

	return &dataNetworks, nil
}
