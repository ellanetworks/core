package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type CreateRouteOptions struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
}

type GetRouteOptions struct {
	ID int64 `json:"id"`
}

type DeleteRouteOptions struct {
	ID int64 `json:"id"`
}

type Route struct {
	ID          int64  `json:"id"`
	Destination string `json:"destination"`
	Gateway     string `json:"gateway"`
	Interface   string `json:"interface"`
	Metric      int    `json:"metric"`
}

type ListRoutesResponse struct {
	Items      []Route `json:"items"`
	Page       int     `json:"page"`
	PerPage    int     `json:"per_page"`
	TotalCount int     `json:"total_count"`
}

// CreateRoute creates a new route with the provided options. The route will be added to the system's underlying kernel routing table.
func (c *Client) CreateRoute(ctx context.Context, opts *CreateRouteOptions) error {
	payload := struct {
		Destination string `json:"destination"`
		Gateway     string `json:"gateway"`
		Interface   string `json:"interface"`
		Metric      int    `json:"metric"`
	}{
		Destination: opts.Destination,
		Gateway:     opts.Gateway,
		Interface:   opts.Interface,
		Metric:      opts.Metric,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/networking/routes",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// GetRoute retrieves a route by ID.
func (c *Client) GetRoute(ctx context.Context, opts *GetRouteOptions) (*Route, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/routes/" + fmt.Sprintf("%d", opts.ID),
	})
	if err != nil {
		return nil, err
	}

	var routeResponse Route

	err = resp.DecodeResult(&routeResponse)
	if err != nil {
		return nil, err
	}

	return &routeResponse, nil
}

// DeleteRoute deletes a route by ID.
func (c *Client) DeleteRoute(ctx context.Context, opts *DeleteRouteOptions) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/networking/routes/" + fmt.Sprintf("%d", opts.ID),
	})
	if err != nil {
		return err
	}

	return nil
}

// ListRoutes lists routes with pagination.
func (c *Client) ListRoutes(ctx context.Context, p *ListParams) (*ListRoutesResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/routes",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var routesResponse ListRoutesResponse

	err = resp.DecodeResult(&routesResponse)
	if err != nil {
		return nil, err
	}

	return &routesResponse, nil
}
