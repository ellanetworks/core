package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type CreateSliceOptions struct {
	Name string `json:"name"`
	Sst  int    `json:"sst"`
	Sd   string `json:"sd,omitempty"`
}

type UpdateSliceOptions struct {
	Sst int    `json:"sst"`
	Sd  string `json:"sd,omitempty"`
}

type GetSliceOptions struct {
	Name string `json:"name"`
}

type DeleteSliceOptions struct {
	Name string `json:"name"`
}

type Slice struct {
	Name string `json:"name"`
	Sst  int    `json:"sst"`
	Sd   string `json:"sd,omitempty"`
}

type ListSlicesResponse struct {
	Items      []Slice `json:"items"`
	Page       int     `json:"page"`
	PerPage    int     `json:"per_page"`
	TotalCount int     `json:"total_count"`
}

// CreateSlice creates a new network slice.
func (c *Client) CreateSlice(ctx context.Context, opts *CreateSliceOptions) error {
	payload := struct {
		Name string `json:"name"`
		Sst  int    `json:"sst"`
		Sd   string `json:"sd,omitempty"`
	}{
		Name: opts.Name,
		Sst:  opts.Sst,
		Sd:   opts.Sd,
	}

	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(payload)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/v1/slices",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// GetSlice retrieves a network slice by name.
func (c *Client) GetSlice(ctx context.Context, opts *GetSliceOptions) (*Slice, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/slices/" + opts.Name,
	})
	if err != nil {
		return nil, err
	}

	var slice Slice

	err = resp.DecodeResult(&slice)
	if err != nil {
		return nil, err
	}

	return &slice, nil
}

// UpdateSlice updates an existing network slice by name.
func (c *Client) UpdateSlice(ctx context.Context, name string, opts *UpdateSliceOptions) error {
	payload := struct {
		Sst int    `json:"sst"`
		Sd  string `json:"sd,omitempty"`
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
		Path:   "api/v1/slices/" + name,
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

// DeleteSlice deletes a network slice by name.
func (c *Client) DeleteSlice(ctx context.Context, opts *DeleteSliceOptions) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/v1/slices/" + opts.Name,
	})
	if err != nil {
		return err
	}

	return nil
}

// ListSlices lists network slices with pagination.
func (c *Client) ListSlices(ctx context.Context, p *ListParams) (*ListSlicesResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/slices",
		Query: url.Values{
			"page":     {fmt.Sprintf("%d", p.Page)},
			"per_page": {fmt.Sprintf("%d", p.PerPage)},
		},
	})
	if err != nil {
		return nil, err
	}

	var slices ListSlicesResponse

	err = resp.DecodeResult(&slices)
	if err != nil {
		return nil, err
	}

	return &slices, nil
}
