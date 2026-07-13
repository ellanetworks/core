// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type CreateDataNetworkOptions struct {
	Name     string `json:"name"`
	IPv4Pool string `json:"ipv4_pool"`
	IPv6Pool string `json:"ipv6_pool,omitempty"`
	DNS      string `json:"dns"`
	Mtu      int32  `json:"mtu"`
}

type UpdateDataNetworkOptions struct {
	Name     string `json:"name"`
	IPv4Pool string `json:"ipv4_pool"`
	IPv6Pool string `json:"ipv6_pool,omitempty"`
	DNS      string `json:"dns"`
	Mtu      int32  `json:"mtu"`
}

type GetDataNetworkOptions struct {
	Name string `json:"name"`
}

type DeleteDataNetworkOptions struct {
	Name string `json:"name"`
}

type ListIPAllocationsOptions struct {
	DataNetworkName string
}

type DataNetworkStatus struct {
	Sessions int `json:"sessions"`
}

type DataNetworkIPAllocation struct {
	PoolSize  int `json:"pool_size"`
	Allocated int `json:"allocated"`
	Available int `json:"available"`
}

type DataNetwork struct {
	Name         string                   `json:"name"`
	IPv4Pool     string                   `json:"ipv4_pool"`
	IPv6Pool     string                   `json:"ipv6_pool,omitempty"`
	DNS          string                   `json:"dns"`
	Mtu          int32                    `json:"mtu"`
	Status       DataNetworkStatus        `json:"status"`
	IPAllocation *DataNetworkIPAllocation `json:"ip_allocation,omitempty"`
}

type IPAllocation struct {
	Address   string `json:"address"`
	IMSI      string `json:"imsi"`
	Type      string `json:"type"`
	SessionID *int   `json:"session_id"`
}

type ListIPAllocationsResponse struct {
	Items      []IPAllocation `json:"items"`
	Page       int            `json:"page"`
	PerPage    int            `json:"per_page"`
	TotalCount int            `json:"total_count"`
}

type ListDataNetworksResponse struct {
	Items      []DataNetwork `json:"items"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalCount int           `json:"total_count"`
}

// CreateDataNetwork creates a new data network with the provided options.
func (c *Client) CreateDataNetwork(ctx context.Context, opts *CreateDataNetworkOptions) error {
	payload := struct {
		Name     string `json:"name"`
		IPv4Pool string `json:"ipv4_pool"`
		IPv6Pool string `json:"ipv6_pool,omitempty"`
		DNS      string `json:"dns"`
		Mtu      int32  `json:"mtu"`
	}{
		Name:     opts.Name,
		IPv4Pool: opts.IPv4Pool,
		IPv6Pool: opts.IPv6Pool,
		DNS:      opts.DNS,
		Mtu:      opts.Mtu,
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

// UpdateDataNetwork updates an existing data network with the provided options.
func (c *Client) UpdateDataNetwork(ctx context.Context, opts *UpdateDataNetworkOptions) error {
	payload := struct {
		Name     string `json:"name"`
		IPv4Pool string `json:"ipv4_pool"`
		IPv6Pool string `json:"ipv6_pool,omitempty"`
		DNS      string `json:"dns"`
		Mtu      int32  `json:"mtu"`
	}{
		Name:     opts.Name,
		IPv4Pool: opts.IPv4Pool,
		IPv6Pool: opts.IPv6Pool,
		DNS:      opts.DNS,
		Mtu:      opts.Mtu,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/networking/data-networks/" + opts.Name,
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// GetDataNetwork retrieves the details of a data network by name.
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

// DeleteDataNetwork deletes a data network by name.
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

// ListDataNetworks lists all data networks with pagination support.
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

type StaticIP struct {
	IMSI        string `json:"imsi"`
	DataNetwork string `json:"data_network"`
	IPVersion   string `json:"ip_version"`
	Address     string `json:"address"`
	Status      string `json:"status"`
	SessionID   *int   `json:"session_id"`
}

type StaticIPList struct {
	Items      []StaticIP `json:"items"`
	Page       int        `json:"page"`
	PerPage    int        `json:"per_page"`
	TotalCount int        `json:"total_count"`
}

type CreateStaticIPOptions struct {
	IMSI    string `json:"imsi"`
	Address string `json:"address"`
}

// ListDataNetworkStaticIps lists the static IP reservations for a data network.
func (c *Client) ListDataNetworkStaticIps(ctx context.Context, dataNetwork string) (*StaticIPList, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/data-networks/" + dataNetwork + "/static-ips",
	})
	if err != nil {
		return nil, err
	}

	var list StaticIPList

	err = resp.DecodeResult(&list)
	if err != nil {
		return nil, err
	}

	return &list, nil
}

// CreateDataNetworkStaticIp pins an address to a subscriber on a data network.
// The IP version is inferred from the address family.
func (c *Client) CreateDataNetworkStaticIp(ctx context.Context, dataNetwork string, opts *CreateStaticIPOptions) error {
	payload := struct {
		IMSI    string `json:"imsi"`
		Address string `json:"address"`
	}{
		IMSI:    opts.IMSI,
		Address: opts.Address,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/networking/data-networks/" + dataNetwork + "/static-ips",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// UpdateDataNetworkStaticIp repins a subscriber's reservation to a new address.
func (c *Client) UpdateDataNetworkStaticIp(ctx context.Context, dataNetwork, imsi, ipVersion, address string) error {
	payload := struct {
		Address string `json:"address"`
	}{
		Address: address,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/networking/data-networks/" + dataNetwork + "/static-ips/" + imsi + "/" + ipVersion,
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// DeleteDataNetworkStaticIp removes a subscriber's static IP reservation.
func (c *Client) DeleteDataNetworkStaticIp(ctx context.Context, dataNetwork, imsi, ipVersion string) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/networking/data-networks/" + dataNetwork + "/static-ips/" + imsi + "/" + ipVersion,
	})
	if err != nil {
		return err
	}

	return nil
}

type FramedRoute struct {
	IMSI string   `json:"imsi"`
	IPv4 []string `json:"ipv4,omitempty"`
	IPv6 []string `json:"ipv6,omitempty"`
}

type FramedRouteList struct {
	Items      []FramedRoute `json:"items"`
	Page       int           `json:"page"`
	PerPage    int           `json:"per_page"`
	TotalCount int           `json:"total_count"`
}

type CreateFramedRouteOptions struct {
	IMSI string   `json:"imsi"`
	IPv4 []string `json:"ipv4,omitempty"`
	IPv6 []string `json:"ipv6,omitempty"`
}

// ListDataNetworkFramedRoutes lists the framed routes on a data network, grouped
// by subscriber.
func (c *Client) ListDataNetworkFramedRoutes(ctx context.Context, dataNetwork string) (*FramedRouteList, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/data-networks/" + dataNetwork + "/framed-routes",
	})
	if err != nil {
		return nil, err
	}

	var list FramedRouteList

	err = resp.DecodeResult(&list)
	if err != nil {
		return nil, err
	}

	return &list, nil
}

// CreateDataNetworkFramedRoute sets a subscriber's framed-route set on a data
// network.
func (c *Client) CreateDataNetworkFramedRoute(ctx context.Context, dataNetwork string, opts *CreateFramedRouteOptions) error {
	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(opts)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/networking/data-networks/" + dataNetwork + "/framed-routes",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// UpdateDataNetworkFramedRoute replaces a subscriber's framed-route set on a data
// network.
func (c *Client) UpdateDataNetworkFramedRoute(ctx context.Context, dataNetwork, imsi string, ipv4, ipv6 []string) error {
	payload := struct {
		IPv4 []string `json:"ipv4,omitempty"`
		IPv6 []string `json:"ipv6,omitempty"`
	}{
		IPv4: ipv4,
		IPv6: ipv6,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/networking/data-networks/" + dataNetwork + "/framed-routes/" + imsi,
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// DeleteDataNetworkFramedRoute removes a subscriber's framed routes on a data
// network.
func (c *Client) DeleteDataNetworkFramedRoute(ctx context.Context, dataNetwork, imsi string) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/networking/data-networks/" + dataNetwork + "/framed-routes/" + imsi,
	})
	if err != nil {
		return err
	}

	return nil
}

// ListIPv4Allocations lists IPv4 allocations for a data network with pagination support.
func (c *Client) ListIPv4Allocations(ctx context.Context, opts *ListIPAllocationsOptions, p *ListParams) (*ListIPAllocationsResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/data-networks/" + opts.DataNetworkName + "/ipv4-allocations",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var allocations ListIPAllocationsResponse

	err = resp.DecodeResult(&allocations)
	if err != nil {
		return nil, err
	}

	return &allocations, nil
}

func (c *Client) ListIPv6Allocations(ctx context.Context, opts *ListIPAllocationsOptions, p *ListParams) (*ListIPAllocationsResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/data-networks/" + opts.DataNetworkName + "/ipv6-allocations",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var allocations ListIPAllocationsResponse

	err = resp.DecodeResult(&allocations)
	if err != nil {
		return nil, err
	}

	return &allocations, nil
}
