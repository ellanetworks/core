// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type CellPosition struct {
	ID                   string   `json:"id"`
	RAT                  string   `json:"rat"`
	Mcc                  string   `json:"mcc"`
	Mnc                  string   `json:"mnc"`
	CellIdentity         string   `json:"cell_identity"`
	GNbID                *string  `json:"gnb_id,omitempty"`
	Latitude             float64  `json:"latitude"`
	Longitude            float64  `json:"longitude"`
	Altitude             *float64 `json:"altitude,omitempty"`
	UncertaintySemiMajor *float64 `json:"uncertainty_semi_major,omitempty"`
	UncertaintySemiMinor *float64 `json:"uncertainty_semi_minor,omitempty"`
	OrientationMajor     *int     `json:"orientation_major,omitempty"`
	Confidence           *int     `json:"confidence,omitempty"`
	Source               string   `json:"source"`
}

type CreateCellPositionOptions struct {
	RAT                  string   `json:"rat"`
	Mcc                  string   `json:"mcc"`
	Mnc                  string   `json:"mnc"`
	CellIdentity         string   `json:"cell_identity"`
	GNbID                *string  `json:"gnb_id,omitempty"`
	Latitude             float64  `json:"latitude"`
	Longitude            float64  `json:"longitude"`
	Altitude             *float64 `json:"altitude,omitempty"`
	UncertaintySemiMajor *float64 `json:"uncertainty_semi_major,omitempty"`
	UncertaintySemiMinor *float64 `json:"uncertainty_semi_minor,omitempty"`
	OrientationMajor     *int     `json:"orientation_major,omitempty"`
	Confidence           *int     `json:"confidence,omitempty"`
}

type UpdateCellPositionOptions struct {
	RAT                  string   `json:"rat"`
	Mcc                  string   `json:"mcc"`
	Mnc                  string   `json:"mnc"`
	CellIdentity         string   `json:"cell_identity"`
	GNbID                *string  `json:"gnb_id,omitempty"`
	Latitude             float64  `json:"latitude"`
	Longitude            float64  `json:"longitude"`
	Altitude             *float64 `json:"altitude,omitempty"`
	UncertaintySemiMajor *float64 `json:"uncertainty_semi_major,omitempty"`
	UncertaintySemiMinor *float64 `json:"uncertainty_semi_minor,omitempty"`
	OrientationMajor     *int     `json:"orientation_major,omitempty"`
	Confidence           *int     `json:"confidence,omitempty"`
}

// ListCellPositions returns all provisioned cell positions. The endpoint
// responds with a bare array and is not paginated.
func (c *Client) ListCellPositions(ctx context.Context) ([]CellPosition, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/beta/cell-positions",
	})
	if err != nil {
		return nil, err
	}

	var positions []CellPosition

	if err := resp.DecodeResult(&positions); err != nil {
		return nil, err
	}

	return positions, nil
}

func (c *Client) GetCellPosition(ctx context.Context, id string) (*CellPosition, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/beta/cell-positions/" + id,
	})
	if err != nil {
		return nil, err
	}

	var position CellPosition

	if err := resp.DecodeResult(&position); err != nil {
		return nil, err
	}

	return &position, nil
}

func (c *Client) CreateCellPosition(ctx context.Context, opts *CreateCellPositionOptions) error {
	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(opts); err != nil {
		return err
	}

	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "POST",
		Path:   "api/beta/cell-positions",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) UpdateCellPosition(ctx context.Context, id string, opts *UpdateCellPositionOptions) error {
	var body bytes.Buffer

	if err := json.NewEncoder(&body).Encode(opts); err != nil {
		return err
	}

	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/beta/cell-positions/" + id,
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}

func (c *Client) DeleteCellPosition(ctx context.Context, id string) error {
	_, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "DELETE",
		Path:   "api/beta/cell-positions/" + id,
	})
	if err != nil {
		return err
	}

	return nil
}
