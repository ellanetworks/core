// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client

import (
	"context"
)

type DiameterPeerStatus struct {
	State      string `json:"state"`
	OriginHost string `json:"originHost,omitempty"`
	Realm      string `json:"realm,omitempty"`
	Since      string `json:"since"`
	Error      string `json:"error,omitempty"`
}

type DiameterNodeIdentity struct {
	NodeID      string `json:"nodeId"`
	DisplayName string `json:"displayName,omitempty"`
	OriginHost  string `json:"originHost"`
	Local       bool   `json:"local"`
}

type DiameterListener struct {
	Enabled   bool     `json:"enabled"`
	Addresses []string `json:"addresses"`
	Port      int      `json:"port,omitempty"`
}

type DiameterPeerOwner struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type DiameterPeer struct {
	ID         string             `json:"id"`
	Role       string             `json:"role"`
	Owner      *DiameterPeerOwner `json:"owner,omitempty"`
	OriginHost string             `json:"originHost,omitempty"`
	Realm      string             `json:"realm,omitempty"`
	Address    string             `json:"address"`
	Port       int                `json:"port,omitempty"`
	Direction  string             `json:"direction"`
	State      string             `json:"state"`
	Since      string             `json:"since"`
	Error      string             `json:"error,omitempty"`
}

type DiameterStatus struct {
	Realm        string                 `json:"realm,omitempty"`
	ServedRealms []string               `json:"servedRealms"`
	Nodes        []DiameterNodeIdentity `json:"nodes"`
	Listener     DiameterListener       `json:"listener"`
	Peers        []DiameterPeer         `json:"peers"`
}

func (c *Client) GetDiameterStatus(ctx context.Context) (*DiameterStatus, error) {
	var status DiameterStatus

	if err := c.integrationsRequest(ctx, "GET", "api/v1/integrations/diameter", nil, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

func (s *DiameterStatus) LocalOriginHost() string {
	for _, n := range s.Nodes {
		if n.Local {
			return n.OriginHost
		}
	}

	return ""
}
