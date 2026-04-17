// Copyright 2026 Ella Networks

package server

import (
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

// newClusterMux builds the HTTP mux served on the cluster port.
// Routes here are protected by mTLS (no JWT auth). The operatorHandler,
// when non-nil, is mounted behind /cluster/proxy/ so followers can
// forward authenticated writes to the leader over the cluster port.
func newClusterMux(dbInstance *db.Database, operatorHandler http.Handler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /cluster/status", ClusterStatus(dbInstance).ServeHTTP)
	mux.HandleFunc("POST /cluster/members", AddClusterMember(dbInstance).ServeHTTP)
	mux.HandleFunc("DELETE /cluster/members/{id}", RemoveClusterMember(dbInstance).ServeHTTP)
	mux.HandleFunc("POST /cluster/members/{id}/promote", PromoteClusterMember(dbInstance).ServeHTTP)

	if operatorHandler != nil {
		mux.Handle("/cluster/proxy/", http.StripPrefix("/cluster/proxy", operatorHandler))
	}

	return mux
}

type clusterNodeStatus struct {
	Role          string `json:"role"`
	NodeID        int    `json:"nodeId"`
	ClusterID     string `json:"clusterId,omitempty"`
	SchemaVersion int    `json:"schemaVersion"`
}

type clusterStatusResponse struct {
	Cluster clusterNodeStatus `json:"cluster"`
}

// ClusterStatus returns the node's Raft role, ID, cluster ID, and
// schema version. Used by peers during discovery and health checks.
func ClusterStatus(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := clusterNodeStatus{
			Role:          dbInstance.RaftState(),
			NodeID:        dbInstance.NodeID(),
			SchemaVersion: db.SchemaVersion(),
		}

		op, err := dbInstance.GetOperator(r.Context())
		if err == nil && op.ClusterID != "" {
			status.ClusterID = op.ClusterID
		}

		writeResponse(r.Context(), w, clusterStatusResponse{Cluster: status}, http.StatusOK, logger.APILog)
	})
}
