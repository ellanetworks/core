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

const (
	ClusterHealthHealthy  = "healthy"
	ClusterHealthDegraded = "degraded"
	ClusterHealthNoLeader = "no_leader"
	ClusterHealthUnknown  = "unknown"
)

type ClusterHealthResponse struct {
	State            string `json:"state"`
	TotalVoters      int    `json:"totalVoters"`
	HealthyVoters    *int   `json:"healthyVoters,omitempty"`
	FailureTolerance *int   `json:"failureTolerance,omitempty"`
}

func GetClusterHealth(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeResponse(r.Context(), w, clusterHealth(r.Context(), dbInstance), http.StatusOK, logger.APILog)
	})
}

func clusterHealth(ctx context.Context, dbInstance *db.Database) ClusterHealthResponse {
	totalVoters := configuredVoters(dbInstance)

	if !dbInstance.HasLeader() {
		return ClusterHealthResponse{
			State:       ClusterHealthNoLeader,
			TotalVoters: totalVoters,
		}
	}

	if !dbInstance.ClusterEnabled() {
		return singleServerHealth(totalVoters)
	}

	state, ok := readAutopilotState(ctx, dbInstance)
	if !ok {
		return ClusterHealthResponse{
			State:       ClusterHealthUnknown,
			TotalVoters: totalVoters,
		}
	}

	return summarizeClusterHealth(state, totalVoters)
}

func singleServerHealth(totalVoters int) ClusterHealthResponse {
	if totalVoters < 1 {
		totalVoters = 1
	}

	healthy := totalVoters
	failureTolerance := 0

	return ClusterHealthResponse{
		State:            ClusterHealthHealthy,
		TotalVoters:      totalVoters,
		HealthyVoters:    &healthy,
		FailureTolerance: &failureTolerance,
	}
}

func configuredVoters(dbInstance *db.Database) int {
	voters := 0

	for _, srv := range dbInstance.RaftServers() {
		if srv.Suffrage == "voter" {
			voters++
		}
	}

	return voters
}

func readAutopilotState(ctx context.Context, dbInstance *db.Database) (AutopilotStateResponse, bool) {
	if dbInstance.IsLeader() {
		state := dbInstance.AutopilotState()
		if state == nil {
			return AutopilotStateResponse{}, false
		}

		return mapAutopilotState(state), true
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

func summarizeClusterHealth(state AutopilotStateResponse, totalVoters int) ClusterHealthResponse {
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

	if len(state.Voters) > 0 {
		totalVoters = len(state.Voters)
	}

	resultState := ClusterHealthHealthy
	if !state.Healthy {
		resultState = ClusterHealthDegraded
	}

	failureTolerance := state.FailureTolerance

	return ClusterHealthResponse{
		State:            resultState,
		TotalVoters:      totalVoters,
		HealthyVoters:    &healthy,
		FailureTolerance: &failureTolerance,
	}
}
