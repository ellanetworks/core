package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
		Path:   "api/v1/routes",
		Body:   &body,
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) GetRoute(ctx context.Context, opts *GetRouteOptions) (*Route, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/routes/" + fmt.Sprintf("%d", opts.ID),
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

func (c *Client) DeleteRoute(ctx context.Context, opts *DeleteRouteOptions) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/routes/" + fmt.Sprintf("%d", opts.ID),
	})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ListRoutes(ctx context.Context) ([]*Route, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/routes",
	})
	if err != nil {
		return nil, err
	}
	var routes []*Route
	err = resp.DecodeResult(&routes)
	if err != nil {
		return nil, err
	}
	return routes, nil
}
