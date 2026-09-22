// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	autopilot "github.com/hashicorp/raft-autopilot"
)

// InternalAutopilotPath is the cluster-mTLS endpoint a follower hits
// to read the leader's autopilot state. Autopilot only runs on the
// leader, so followers cannot answer this read locally.
const InternalAutopilotPath = "/cluster/internal/autopilot"

// AutopilotServerResponse is the per-peer live state on the wire. It is
// mapped from autopilot.ServerState so the library struct never leaks
// into the public API.
type AutopilotServerResponse struct {
	NodeID          pki.NodeID `json:"nodeId"`
	RaftAddress     string     `json:"raftAddress"`
	NodeStatus      string     `json:"nodeStatus"`
	Healthy         bool       `json:"healthy"`
	IsLeader        bool       `json:"isLeader"`
	HasVotingRights bool       `json:"hasVotingRights"`
	StableSince     string     `json:"stableSince,omitempty"`
}

// AutopilotStateResponse is the cluster-wide live state on the wire.
type AutopilotStateResponse struct {
	Healthy          bool                      `json:"healthy"`
	FailureTolerance int                       `json:"failureTolerance"`
	LeaderNodeID     pki.NodeID                `json:"leaderNodeId"`
	Voters           []pki.NodeID              `json:"voters"`
	Servers          []AutopilotServerResponse `json:"servers"`
}

// GetAutopilotState serves the live autopilot state. Autopilot only runs
// on the leader; on a follower the handler issues a one-shot read
// against the leader's cluster mTLS port. An empty response is served
// during the cold-start window immediately after leadership
// acquisition, before the first autopilot tick has published a state.
func GetAutopilotState(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if dbInstance.IsLeader() || !dbInstance.ClusterEnabled() {
			state := dbInstance.AutopilotState()
			resp := mapAutopilotState(state)
			writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)

			return
		}

		leaderResp, err := dbInstance.DoLeaderRequest(r.Context(), http.MethodGet, InternalAutopilotPath, nil, "")
		if err != nil {
			writeError(r.Context(), w, http.StatusServiceUnavailable, "Failed to read autopilot state from leader", err, logger.APILog)
			return
		}

		if leaderResp.StatusCode == http.StatusMisdirectedRequest {
			w.Header().Set("Retry-After", "1")
			writeError(r.Context(), w, http.StatusServiceUnavailable, "Leadership changed while reading autopilot state, retry shortly", nil, logger.APILog)

			return
		}

		if leaderResp.StatusCode != http.StatusOK {
			writeError(r.Context(), w, http.StatusBadGateway, "Leader returned non-OK status reading autopilot state", errors.New(http.StatusText(leaderResp.StatusCode)), logger.APILog)
			return
		}

		var resp AutopilotStateResponse
		if err := json.Unmarshal(leaderResp.Body, &resp); err != nil {
			writeError(r.Context(), w, http.StatusBadGateway, "Failed to decode autopilot state from leader", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

// ClusterAutopilotState serves the autopilot state on the cluster mTLS
// port. Followers call this when a public-API request reaches them.
// The response body is the bare AutopilotStateResponse (no envelope)
// so the follower can re-wrap it in its own response envelope.
func ClusterAutopilotState(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := dbInstance.AutopilotState()
		resp := mapAutopilotState(state)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func mapAutopilotState(state *autopilot.State) AutopilotStateResponse {
	if state == nil {
		return AutopilotStateResponse{
			Voters:  []pki.NodeID{},
			Servers: []AutopilotServerResponse{},
		}
	}

	voters := make([]pki.NodeID, 0, len(state.Voters))

	for _, id := range state.Voters {
		voters = append(voters, pki.NodeID(id))
	}

	sort.Slice(voters, func(i, j int) bool { return voters[i] < voters[j] })

	leaderID := pki.NodeID(state.Leader)

	servers := make([]AutopilotServerResponse, 0, len(state.Servers))
	for id, srv := range state.Servers {
		nodeID := pki.NodeID(id)

		item := AutopilotServerResponse{
			NodeID:          nodeID,
			RaftAddress:     string(srv.Server.Address),
			NodeStatus:      string(srv.Server.NodeStatus),
			Healthy:         srv.Health.Healthy,
			IsLeader:        nodeID == leaderID,
			HasVotingRights: srv.HasVotingRights(),
		}

		if !srv.Health.StableSince.IsZero() {
			item.StableSince = srv.Health.StableSince.UTC().Format(time.RFC3339)
		}

		servers = append(servers, item)
	}

	sort.Slice(servers, func(i, j int) bool {
		return servers[i].NodeID < servers[j].NodeID
	})

	return AutopilotStateResponse{
		Healthy:          state.Healthy,
		FailureTolerance: state.FailureTolerance,
		LeaderNodeID:     leaderID,
		Voters:           voters,
		Servers:          servers,
	}
}
