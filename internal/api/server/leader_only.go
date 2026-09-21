// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
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

		writeError(r.Context(), w, http.StatusMisdirectedRequest,
			notLeaderMessage(r.Context(), dbInstance), nil, logger.APILog)
	})
}

func notLeaderMessage(ctx context.Context, dbInstance clusterLeadership) string {
	const prefix = "This node is not the cluster leader"

	_, leaderNodeID := dbInstance.LeaderAddressAndID()
	if leaderNodeID == "" {
		return prefix + "; no leader is currently elected, retry shortly"
	}

	member, err := dbInstance.GetClusterMember(ctx, leaderNodeID)
	if err != nil || member == nil {
		return fmt.Sprintf("%s; retry against node %s", prefix, leaderNodeID)
	}

	target := leaderNodeID
	if member.DisplayName != "" {
		target = fmt.Sprintf("%s (%s)", member.DisplayName, leaderNodeID)
	}

	if member.APIAddress == "" {
		return fmt.Sprintf("%s; retry against node %s", prefix, target)
	}

	return fmt.Sprintf("%s; retry against node %s at %s", prefix, target, member.APIAddress)
}
