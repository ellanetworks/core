// Copyright 2026 Ella Networks

package server

import (
	"context"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// dropPinForRemovedNode is the v12 equivalent of cert revocation: in
// the fingerprint-pinning model, removing a node means deleting its
// row in cluster_node_certs. After the row is replicated out, peer
// listeners refuse the removed node's handshakes immediately. This
// function also tears down any in-flight connections whose peer cert
// matches the dropped pin.
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
		// No pin for that nodeID — already removed, or never registered.
		return
	}

	if err := dbInstance.DeleteClusterNodeCert(ctx, nodeID); err != nil {
		logger.APILog.Warn("revocation: delete pin failed",
			zap.Int("nodeId", nodeID), zap.Error(err))

		return
	}

	// Refresh our own pin cache immediately so handshakes on this
	// leader reject the removed cert without waiting for the periodic
	// refresher. Followers still lag by up to that interval.
	nudgePinCache(ctx)

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
