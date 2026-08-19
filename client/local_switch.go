// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"bytes"
	"context"
	"encoding/json"
)

type GetLocalSwitchInfoResponse struct {
	Enabled bool `json:"enabled"`
}

type UpdateLocalSwitchInfoOptions struct {
	Enabled bool `json:"enabled"`
}

func (c *Client) GetLocalSwitchInfo(ctx context.Context) (*GetLocalSwitchInfoResponse, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/local-switch",
	})
	if err != nil {
		return nil, err
	}

	var localSwitchResponse GetLocalSwitchInfoResponse

	err = resp.DecodeResult(&localSwitchResponse)
	if err != nil {
		return nil, err
	}

	return &localSwitchResponse, nil
}

func (c *Client) UpdateLocalSwitchInfo(ctx context.Context, opts *UpdateLocalSwitchInfoOptions) error {
	var body bytes.Buffer

	err := json.NewEncoder(&body).Encode(opts)
	if err != nil {
		return err
	}

	_, err = c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "PUT",
		Path:   "api/v1/networking/local-switch",
		Body:   &body,
	})
	if err != nil {
		return err
	}

	return nil
}
