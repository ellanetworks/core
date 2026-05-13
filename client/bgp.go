package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type BGPRejectedPrefix struct {
	Prefix      string `json:"prefix"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

type GetBGPSettingsResponse struct {
	Enabled          bool                `json:"enabled"`
	LocalAS          int                 `json:"localAS"`
	RouterID         string              `json:"routerID"`
	ListenAddress    string              `json:"listenAddress"`
	RejectedPrefixes []BGPRejectedPrefix `json:"rejectedPrefixes"`
}

type UpdateBGPSettingsOptions struct {
	Enabled       bool   `json:"enabled"`
	LocalAS       int    `json:"localAS"`
	RouterID      string `json:"routerID"`
	ListenAddress string `json:"listenAddress"`
}

type BGPImportPrefix struct {
	Prefix    string `json:"prefix"`
	MaxLength int    `json:"maxLength"`
}

type BGPPeer struct {
	ID               int               `json:"id"`
	Address          string            `json:"address"`
	RemoteAS         int               `json:"remoteAS"`
	HoldTime         int               `json:"holdTime"`
	HasPassword      bool              `json:"hasPassword"`
	Description      string            `json:"description"`
	ImportPrefixes   []BGPImportPrefix `json:"importPrefixes"`
	State            string            `json:"state,omitempty"`
	Uptime           string            `json:"uptime,omitempty"`
	PrefixesSent     int               `json:"prefixesSent,omitempty"`
	PrefixesReceived int               `json:"prefixesReceived,omitempty"`
	PrefixesAccepted int               `json:"prefixesAccepted,omitempty"`
}

type ListBGPPeersResponse struct {
	Items      []BGPPeer `json:"items"`
	Page       int       `json:"page"`
	PerPage    int       `json:"per_page"`
	TotalCount int       `json:"total_count"`
}

type GetBGPPeerOptions struct {
	ID int `json:"id"`
}

type DeleteBGPPeerOptions struct {
	ID int `json:"id"`
}

type CreateBGPPeerOptions struct {
	Address        string            `json:"address"`
	RemoteAS       int               `json:"remoteAS"`
	HoldTime       int               `json:"holdTime"`
	Password       string            `json:"password"`
	Description    string            `json:"description"`
	ImportPrefixes []BGPImportPrefix `json:"importPrefixes"`
}

// UpdateBGPPeerOptions updates a BGP peer. Password is a pointer so that
// callers can distinguish "leave unchanged" (nil) from "clear" (empty string).
type UpdateBGPPeerOptions struct {
	ID             int               `json:"-"`
	Address        string            `json:"address"`
	RemoteAS       int               `json:"remoteAS"`
	HoldTime       int               `json:"holdTime"`
	Password       *string           `json:"password,omitempty"`
	Description    string            `json:"description"`
	ImportPrefixes []BGPImportPrefix `json:"importPrefixes"`
}

type BGPAdvertisedRoute struct {
	Subscriber string `json:"subscriber"`
	Prefix     string `json:"prefix"`
	NextHop    string `json:"nextHop"`
}

type BGPLearnedRoute struct {
	Prefix  string `json:"prefix"`
	NextHop string `json:"nextHop"`
	Peer    string `json:"peer"`
}

type BGPAdvertisedRoutesResponse struct {
	Routes []BGPAdvertisedRoute `json:"routes"`
}

type BGPLearnedRoutesResponse struct {
	Routes []BGPLearnedRoute `json:"routes"`
}

// GetBGPSettings retrieves the current BGP speaker configuration.
func (c *Client) GetBGPSettings(ctx context.Context) (*GetBGPSettingsResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/bgp",
	})
	if err != nil {
		return nil, err
	}

	var settings GetBGPSettingsResponse

	err = resp.DecodeResult(&settings)
	if err != nil {
		return nil, err
	}

	return &settings, nil
}

// UpdateBGPSettings updates the BGP speaker configuration.
func (c *Client) UpdateBGPSettings(ctx context.Context, opts *UpdateBGPSettingsOptions) error {
	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(opts)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/networking/bgp",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// ListBGPPeers lists configured BGP peers with pagination, enriched with live
// session state when the BGP speaker is running.
func (c *Client) ListBGPPeers(ctx context.Context, p *ListParams) (*ListBGPPeersResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/bgp/peers",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var peersResponse ListBGPPeersResponse

	err = resp.DecodeResult(&peersResponse)
	if err != nil {
		return nil, err
	}

	return &peersResponse, nil
}

// GetBGPPeer retrieves a single BGP peer by ID.
func (c *Client) GetBGPPeer(ctx context.Context, opts *GetBGPPeerOptions) (*BGPPeer, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   fmt.Sprintf("api/v1/networking/bgp/peers/%d", opts.ID),
	})
	if err != nil {
		return nil, err
	}

	var peer BGPPeer

	err = resp.DecodeResult(&peer)
	if err != nil {
		return nil, err
	}

	return &peer, nil
}

// CreateBGPPeer creates a new BGP peer.
func (c *Client) CreateBGPPeer(ctx context.Context, opts *CreateBGPPeerOptions) error {
	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(opts)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/networking/bgp/peers",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// UpdateBGPPeer updates an existing BGP peer.
func (c *Client) UpdateBGPPeer(ctx context.Context, opts *UpdateBGPPeerOptions) error {
	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(opts)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   fmt.Sprintf("api/v1/networking/bgp/peers/%d", opts.ID),
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// DeleteBGPPeer deletes a BGP peer by ID.
func (c *Client) DeleteBGPPeer(ctx context.Context, opts *DeleteBGPPeerOptions) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   fmt.Sprintf("api/v1/networking/bgp/peers/%d", opts.ID),
	})
	if err != nil {
		return err
	}

	return nil
}

// GetBGPAdvertisedRoutes returns routes the local speaker is advertising.
func (c *Client) GetBGPAdvertisedRoutes(ctx context.Context) (*BGPAdvertisedRoutesResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/bgp/advertised-routes",
	})
	if err != nil {
		return nil, err
	}

	var routesResponse BGPAdvertisedRoutesResponse

	err = resp.DecodeResult(&routesResponse)
	if err != nil {
		return nil, err
	}

	return &routesResponse, nil
}

// GetBGPLearnedRoutes returns BGP-learned routes installed in the kernel.
func (c *Client) GetBGPLearnedRoutes(ctx context.Context) (*BGPLearnedRoutesResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/bgp/learned-routes",
	})
	if err != nil {
		return nil, err
	}

	var routesResponse BGPLearnedRoutesResponse

	err = resp.DecodeResult(&routesResponse)
	if err != nil {
		return nil, err
	}

	return &routesResponse, nil
}
