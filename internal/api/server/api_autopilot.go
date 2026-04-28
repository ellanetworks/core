package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
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
	NodeID          int    `json:"nodeId"`
	RaftAddress     string `json:"raftAddress"`
	NodeStatus      string `json:"nodeStatus"`
	Healthy         bool   `json:"healthy"`
	IsLeader        bool   `json:"isLeader"`
	HasVotingRights bool   `json:"hasVotingRights"`
	StableSince     string `json:"stableSince,omitempty"`
}

// AutopilotStateResponse is the cluster-wide live state on the wire.
type AutopilotStateResponse struct {
	Healthy          bool                      `json:"healthy"`
	FailureTolerance int                       `json:"failureTolerance"`
	LeaderNodeID     int                       `json:"leaderNodeId"`
	Voters           []int                     `json:"voters"`
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
		if !dbInstance.IsLeader() {
			http.Error(w, "not the leader", http.StatusMisdirectedRequest)
			return
		}

		state := dbInstance.AutopilotState()
		resp := mapAutopilotState(state)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
}

func mapAutopilotState(state *autopilot.State) AutopilotStateResponse {
	if state == nil {
		return AutopilotStateResponse{
			Voters:  []int{},
			Servers: []AutopilotServerResponse{},
		}
	}

	voters := make([]int, 0, len(state.Voters))

	for _, id := range state.Voters {
		if n, err := parseRaftServerID(id); err == nil {
			voters = append(voters, n)
		}
	}

	sort.Ints(voters)

	leaderID, _ := parseRaftServerID(state.Leader)

	servers := make([]AutopilotServerResponse, 0, len(state.Servers))
	for id, srv := range state.Servers {
		nodeID, err := parseRaftServerID(id)
		if err != nil {
			continue
		}

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

func parseRaftServerID(id raft.ServerID) (int, error) {
	return strconv.Atoi(string(id))
}
