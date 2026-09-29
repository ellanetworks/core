// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smsf"
)

type DiameterNode interface {
	Identity(ctx context.Context) (smsf.Identity, error)
	Peers() []smsf.PeerStatus
}

type DiameterPeer struct {
	Role    string `json:"role"`
	Host    string `json:"host,omitempty"`
	Realm   string `json:"realm,omitempty"`
	Address string `json:"address"`
	Port    int    `json:"port"`
	State   string `json:"state"`
	Since   string `json:"since"`
}

type DiameterStatus struct {
	Host  string         `json:"host,omitempty"`
	Realm string         `json:"realm,omitempty"`
	Peers []DiameterPeer `json:"peers"`
}

func GetDiameterStatus(node DiameterNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := DiameterStatus{Peers: []DiameterPeer{}}

		if node == nil {
			writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
			return
		}

		if identity, err := node.Identity(r.Context()); err == nil {
			resp.Host = identity.Host
			resp.Realm = identity.Realm
		}

		for _, p := range node.Peers() {
			resp.Peers = append(resp.Peers, DiameterPeer{
				Role:    p.Role,
				Host:    p.Host,
				Realm:   p.Realm,
				Address: p.Address.Addr().String(),
				Port:    int(p.Address.Port()),
				State:   p.State.String(),
				Since:   p.Since.UTC().Format(time.RFC3339),
			})
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}
