// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type ClusterHealthResponse struct {
	Enabled          bool `json:"enabled"`
	HasLeader        bool `json:"hasLeader"`
	Healthy          bool `json:"healthy"`
	HealthyVoters    int  `json:"healthyVoters"`
	TotalVoters      int  `json:"totalVoters"`
	FailureTolerance int  `json:"failureTolerance"`
}

func GetClusterHealth(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !dbInstance.ClusterEnabled() {
			writeResponse(r.Context(), w, ClusterHealthResponse{}, http.StatusOK, logger.APILog)

			return
		}

		state, ok := readAutopilotState(r.Context(), dbInstance)
		if !ok || state.LeaderNodeID == "" {
			writeResponse(r.Context(), w, ClusterHealthResponse{Enabled: true}, http.StatusOK, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, summarizeClusterHealth(state), http.StatusOK, logger.APILog)
	})
}

func readAutopilotState(ctx context.Context, dbInstance *db.Database) (AutopilotStateResponse, bool) {
	if dbInstance.IsLeader() {
		return mapAutopilotState(dbInstance.AutopilotState()), true
	}

	leaderResp, err := dbInstance.DoLeaderRequest(ctx, http.MethodGet, InternalAutopilotPath, nil, "")
	if err != nil || leaderResp.StatusCode != http.StatusOK {
		return AutopilotStateResponse{}, false
	}

	var state AutopilotStateResponse
	if err := json.Unmarshal(leaderResp.Body, &state); err != nil {
		return AutopilotStateResponse{}, false
	}

	return state, true
}

func summarizeClusterHealth(state AutopilotStateResponse) ClusterHealthResponse {
	voters := make(map[string]struct{}, len(state.Voters))
	for _, id := range state.Voters {
		voters[string(id)] = struct{}{}
	}

	healthy := 0

	for _, srv := range state.Servers {
		if _, isVoter := voters[string(srv.NodeID)]; !isVoter {
			continue
		}

		if srv.Healthy {
			healthy++
		}
	}

	return ClusterHealthResponse{
		Enabled:          true,
		HasLeader:        true,
		Healthy:          state.Healthy,
		HealthyVoters:    healthy,
		TotalVoters:      len(state.Voters),
		FailureTolerance: state.FailureTolerance,
	}
}
