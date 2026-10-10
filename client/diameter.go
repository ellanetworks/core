// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
)

type DiameterPeer struct {
	Role    string `json:"role"`
	Host    string `json:"host,omitempty"`
	Realm   string `json:"realm,omitempty"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	State   string `json:"state"`
	Since   string `json:"since"`
	Error   string `json:"error,omitempty"`
}

type DiameterStatus struct {
	Host  string         `json:"host,omitempty"`
	Realm string         `json:"realm,omitempty"`
	Peers []DiameterPeer `json:"peers"`
}

func (c *Client) GetDiameterStatus(ctx context.Context) (*DiameterStatus, error) {
	resp, err := c.Requester.Do(ctx, &RequestOptions{
		Type:   SyncRequest,
		Method: "GET",
		Path:   "api/v1/networking/diameter",
	})
	if err != nil {
		return nil, err
	}

	var status DiameterStatus

	if err := resp.DecodeResult(&status); err != nil {
		return nil, err
	}

	return &status, nil
}
