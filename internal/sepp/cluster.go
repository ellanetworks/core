// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

const (
	ClusterStatusPath    = "/cluster/internal/sepp/status"
	ClusterHandshakePath = "/cluster/internal/sepp/handshake"

	maxClusterBodyBytes = 4 << 10
)

type HandshakeRequest struct {
	PartnerID string `json:"partnerId"`
}

func (m *Manager) ClusterHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET "+ClusterStatusPath, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, m.Status())
	})

	mux.HandleFunc("POST "+ClusterHandshakePath, func(w http.ResponseWriter, r *http.Request) {
		var req HandshakeRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, maxClusterBodyBytes)).Decode(&req); err != nil || req.PartnerID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}

		st, err := m.Rehandshake(req.PartnerID)

		switch {
		case errors.Is(err, ErrUnknownPartner):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		case err != nil:
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		default:
			writeJSON(w, http.StatusOK, st)
		}
	})

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", wire.ContentTypeJSON)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func DecodeNodeStatus(body []byte) (NodeStatus, error) {
	var st NodeStatus
	if err := json.Unmarshal(body, &st); err != nil {
		return NodeStatus{}, err
	}

	return st, nil
}

func DecodeContextStatus(body []byte) (ContextStatus, error) {
	var st ContextStatus
	if err := json.Unmarshal(body, &st); err != nil {
		return ContextStatus{}, err
	}

	return st, nil
}
