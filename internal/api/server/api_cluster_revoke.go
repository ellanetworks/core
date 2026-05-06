// Copyright 2026 Ella Networks

package server

import (
	"context"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// dropPinForRemovedNode deletes the removed node's
// cluster_node_certs row and closes any in-flight connections
// whose peer cert matches the dropped pin. Once the deletion
// replicates, peer listeners reject the removed node's
// handshakes.
func dropPinForRemovedNode(ctx context.Context, dbInstance *db.Database, ln *listener.Listener, nodeID int) {
	rows, err := dbInstance.ListClusterNodeCerts(ctx)
	if err != nil {
		logger.APILog.Warn("revocation: list pins failed",
			zap.Int("nodeId", nodeID), zap.Error(err))

		return
	}

	var fingerprint string

	for _, r := range rows {
		if r.NodeID == nodeID {
			fingerprint = r.Fingerprint
			break
		}
	}

	if fingerprint == "" {
		// nodeID has no pin: either previously removed or never registered.
		return
	}

	if err := dbInstance.DeleteClusterNodeCert(ctx, nodeID); err != nil {
		logger.APILog.Warn("revocation: delete pin failed",
			zap.Int("nodeId", nodeID), zap.Error(err))

		return
	}

	if ln == nil {
		return
	}

	if closed := ln.CloseByPeerFingerprint(fingerprint); closed > 0 {
		logger.APILog.Info("revocation: closed active cluster connections after member removal",
			zap.Int("nodeId", nodeID),
			zap.String("fingerprint", fingerprint),
			zap.Int("closed", closed))
	}
}
