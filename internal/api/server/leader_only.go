// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/ellanetworks/core/internal/raft"
	"go.uber.org/zap"
)

type clusterLeadership interface {
	ClusterEnabled() bool
	IsLeader() bool
	LeaderAddressAndID() (string, string)
	GetClusterMember(ctx context.Context, nodeID string) (*db.ClusterMember, error)
}

func LeaderOnly(dbInstance clusterLeadership, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !dbInstance.ClusterEnabled() || dbInstance.IsLeader() {
			next.ServeHTTP(w, r)
			return
		}

		resp := notLeaderBody(r.Context(), dbInstance)

		body, err := json.Marshal(&resp)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to marshal response", err, logger.APILog)
			return
		}

		logger.From(r.Context(), logger.APILog).Warn(resp.Error,
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
		)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMisdirectedRequest)

		if _, err := w.Write(body); err != nil {
			logger.APILog.Error("Failed to write response", zap.Error(err))
		}
	})
}

func notLeaderBody(ctx context.Context, dbInstance clusterLeadership) raft.NotLeaderBody {
	const prefix = "This node is not the cluster leader"

	leaderAddr, leaderNodeID := dbInstance.LeaderAddressAndID()
	if leaderNodeID == "" {
		return raft.NotLeaderBody{Error: prefix + "; no leader is currently elected, retry shortly"}
	}

	resp := raft.NotLeaderBody{
		Error:         fmt.Sprintf("%s; retry against node %s", prefix, leaderNodeID),
		LeaderNodeID:  pki.NodeID(leaderNodeID),
		LeaderAddress: leaderAddr,
	}

	member, err := dbInstance.GetClusterMember(ctx, leaderNodeID)
	if err != nil || member == nil {
		return resp
	}

	target := leaderNodeID
	if member.DisplayName != "" {
		target = fmt.Sprintf("%s (%s)", member.DisplayName, leaderNodeID)
	}

	if member.APIAddress == "" {
		resp.Error = fmt.Sprintf("%s; retry against node %s", prefix, target)
		return resp
	}

	resp.Error = fmt.Sprintf("%s; retry against node %s at %s", prefix, target, member.APIAddress)
	resp.LeaderAPIAddress = member.APIAddress

	return resp
}
