// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"net"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// dropPinForRemovedNode deletes the removed node's
// cluster_node_certs row and closes any in-flight connections
// whose peer cert matches the dropped pin, except the one carrying a
// forwarded request, which closes after its response. Once the deletion
// replicates, peer listeners reject the removed node's
// handshakes.
func dropPinForRemovedNode(ctx context.Context, dbInstance *db.Database, ln *listener.Listener, nodeID string) {
	ctx = context.WithoutCancel(ctx)

	var keep net.Conn

	if fr := forwardedFromContext(ctx); fr != nil {
		keep = fr.conn

		if fr.peerNodeID == nodeID {
			fr.closeConn.Store(true)
		}
	}

	rows, err := dbInstance.ListClusterNodeCerts(ctx)
	if err != nil {
		logger.APILog.Warn("revocation: list pins failed",
			zap.String("node_id", nodeID), zap.Error(err))

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
			zap.String("node_id", nodeID), zap.Error(err))

		return
	}

	if ln == nil {
		return
	}

	if closed := ln.CloseByPeerFingerprintExcept(fingerprint, keep); closed > 0 {
		logger.APILog.Info("revocation: closed active cluster connections after member removal",
			zap.String("node_id", nodeID),
			zap.String("fingerprint", fingerprint),
			zap.Int("closed", closed))
	}
}
