package server

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	autopilot "github.com/hashicorp/raft-autopilot"
)

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
	LastContactMs   int64  `json:"lastContactMs"`
	LastTerm        uint64 `json:"lastTerm"`
	LastIndex       uint64 `json:"lastIndex"`
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
// on the leader, so followers proxy the request via LeaderProxyMiddleware.
// On the leader, an empty response is served during the cold-start window
// immediately after leadership acquisition, before the first autopilot
// tick has published a state.
func GetAutopilotState(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := dbInstance.AutopilotState()
		resp := mapAutopilotState(state)
		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
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
			IsLeader:        srv.Server.IsLeader,
			HasVotingRights: srv.HasVotingRights(),
			LastContactMs:   srv.Stats.LastContact.Milliseconds(),
			LastTerm:        srv.Stats.LastTerm,
			LastIndex:       srv.Stats.LastIndex,
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
