// Copyright 2026 Ella Networks

package server

import (
	"context"
	"math/big"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

// revokeIssuedCertsForRemovedNode inserts revocation rows for every
// non-expired leaf ever issued to nodeID and closes any live cluster
// connections whose peer certificate carries one of those serials.
//
// Best-effort: any error is logged but does not roll back the prior
// RemoveClusterMember steps — the node is already out of Raft and the
// cluster-members row is gone, so the worst case is that residual
// leaves expire on their own within pki.DefaultLeafTTL.
func revokeIssuedCertsForRemovedNode(ctx context.Context, dbInstance *db.Database, ln *listener.Listener, nodeID int) {
	issued, err := dbInstance.ListActiveIssuedCertsByNode(ctx, nodeID)
	if err != nil {
		logger.APILog.Warn("revocation: list active issued certs failed",
			zap.Int("nodeId", nodeID), zap.Error(err))

		return
	}

	if len(issued) == 0 {
		return
	}

	now := time.Now()
	// Keep revocations until after the worst-case leaf expiry so
	// any cached leaves already in flight remain rejected.
	purgeAfter := now.Add(pki.MaxLeafTTL + time.Hour)

	for _, c := range issued {
		if err := dbInstance.InsertRevokedCert(ctx, &db.ClusterRevokedCert{
			Serial:     c.Serial,
			NodeID:     nodeID,
			RevokedAt:  now.Unix(),
			Reason:     "cluster member removed",
			PurgeAfter: purgeAfter.Unix(),
		}); err != nil {
			logger.APILog.Warn("revocation: insert revoked cert failed",
				zap.Int("nodeId", nodeID), zap.Int64("serial", c.Serial), zap.Error(err))
		}
	}

	// After replicating revocation rows, refresh our own revocation
	// cache immediately so new handshakes on this leader reject the
	// stale leaves without waiting for the 30 s periodic refresher.
	// Follower caches still lag by up to that interval.
	refreshLocalRevocations(ctx)

	if ln == nil {
		return
	}

	// Close any currently-open cluster connections whose peer cert
	// matches one of the revoked serials.
	for _, c := range issued {
		closed := ln.CloseByPeerSerial(big.NewInt(c.Serial))
		if closed > 0 {
			logger.APILog.Info("revocation: closed active cluster connections after member removal",
				zap.Int("nodeId", nodeID), zap.Int64("serial", c.Serial), zap.Int("closed", closed))
		}
	}
}
