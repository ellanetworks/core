// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/ellanetworks/core/internal/cluster/joinreq"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	maxClusterJoinRequestBytes = 8 << 10

	minClusterJoinTokenLen = 64
)

var joinCoordinator atomic.Pointer[joinreq.Coordinator]

func SetJoinCoordinator(c *joinreq.Coordinator) {
	joinCoordinator.Store(c)
}

func loadJoinCoordinator() *joinreq.Coordinator {
	return joinCoordinator.Load()
}

type ClusterJoinRequest struct {
	Token         string   `json:"token"`
	SeedAddresses []string `json:"seedAddresses"`
	Suffrage      string   `json:"suffrage,omitempty"`
}

type ClusterJoinStatusResponse struct {
	State string `json:"state"`
	Error string `json:"error,omitempty"`
}

func validateClusterJoinRequest(req *ClusterJoinRequest) error {
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return errors.New("token is required")
	}

	if strings.ContainsAny(token, " \t\r\n") {
		return errors.New("token must not contain whitespace")
	}

	if len(token) < minClusterJoinTokenLen {
		return fmt.Errorf("token is too short (%d chars, expected >=%d); looks truncated", len(token), minClusterJoinTokenLen)
	}

	req.Token = token

	if len(req.SeedAddresses) == 0 {
		return errors.New("seedAddresses must contain at least one address of a node already in the cluster")
	}

	for i, addr := range req.SeedAddresses {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			if strings.Contains(addr, "://") {
				return fmt.Errorf("seedAddresses[%d] %q looks like a URL; seed addresses must be host:port (e.g. 10.0.0.1:7000)", i, addr)
			}

			return fmt.Errorf("seedAddresses[%d] %q is not a valid host:port: %w", i, addr, err)
		}
	}

	switch req.Suffrage {
	case "", "voter", "nonvoter":
	default:
		return fmt.Errorf("suffrage must be \"voter\" or \"nonvoter\", got %q", req.Suffrage)
	}

	return nil
}

func ClusterJoin() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		coord := loadJoinCoordinator()
		if coord == nil {
			writeError(r.Context(), w, http.StatusConflict, joinreq.ErrNotAccepting.Error(), nil, logger.APILog)
			return
		}

		var req ClusterJoinRequest

		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxClusterJoinRequestBytes)).Decode(&req); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if err := validateClusterJoinRequest(&req); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), err, logger.APILog)
			return
		}

		err := coord.Submit(joinreq.Request{
			Mode:          joinreq.ModeJoin,
			Token:         req.Token,
			SeedAddresses: req.SeedAddresses,
			Suffrage:      req.Suffrage,
		})
		if err != nil {
			writeError(r.Context(), w, http.StatusConflict, err.Error(), err, logger.APILog)
			return
		}

		logger.APILog.Info("Cluster join requested over the API",
			zap.Strings("seed_addresses", req.SeedAddresses),
			zap.String("suffrage", req.Suffrage),
		)

		writeResponse(r.Context(), w, statusResponse(coord), http.StatusAccepted, logger.APILog)
	})
}

func GetClusterJoinStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		coord := loadJoinCoordinator()
		if coord == nil {
			writeResponse(r.Context(), w, ClusterJoinStatusResponse{State: string(joinreq.StateUnavailable)}, http.StatusOK, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, statusResponse(coord), http.StatusOK, logger.APILog)
	})
}

func statusResponse(coord *joinreq.Coordinator) ClusterJoinStatusResponse {
	st := coord.Status()

	return ClusterJoinStatusResponse{State: string(st.State), Error: st.Error}
}
