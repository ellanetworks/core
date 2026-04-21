// Copyright 2026 Ella Networks

package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// renewFailureBackoff is the delay between retry attempts after a
// renewal failure (missing leader, dial error, HTTP error, etc.). Short
// enough that a transient leader transition does not push a node past
// its hard expiry.
const renewFailureBackoff = 30 * time.Second

// runLeafRenewer is the per-node leaf renewal worker. Every node runs
// one; it blocks until its current leaf hits the renewal window picked
// by Agent.PickRenewAt, then POSTs a fresh CSR to the leader's
// /cluster/pki/renew over the mTLS cluster listener. On success it
// reschedules; on failure it retries with renewFailureBackoff.
//
// The worker is defensive about the leaderless window: if no leader is
// known yet, it simply retries. A follower that becomes leader after
// losing its leaf still renews itself through its own listener by
// dialling its own advertise address (Dial is local-loopback safe).
func runLeafRenewer(ctx context.Context, pki *pkiState, ln *listener.Listener, dbInstance *db.Database) {
	for {
		leaf := pki.agent.Leaf()
		if leaf == nil || leaf.Leaf == nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(renewFailureBackoff):
				continue
			}
		}

		now := time.Now()
		renewAt := pki.agent.PickRenewAt(now)

		wait := time.Until(renewAt)
		if wait < 0 {
			wait = 0
		}

		logger.EllaLog.Info("leaf renewal scheduled",
			zap.Time("renewAt", renewAt),
			zap.Time("notAfter", leaf.Leaf.NotAfter))

		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		if err := renewOnce(ctx, pki, ln, dbInstance); err != nil {
			logger.EllaLog.Warn("leaf renewal failed; retrying",
				zap.Error(err),
				zap.Duration("retryIn", renewFailureBackoff))

			select {
			case <-ctx.Done():
				return
			case <-time.After(renewFailureBackoff):
			}

			continue
		}

		logger.EllaLog.Info("leaf renewal succeeded")
	}
}

func renewOnce(ctx context.Context, pki *pkiState, ln *listener.Listener, dbInstance *db.Database) error {
	leaderAddr, leaderID := dbInstance.LeaderAddressAndID()
	if leaderAddr == "" || leaderID == 0 {
		return fmt.Errorf("no leader yet")
	}

	renewCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return pki.agent.RenewFlow(renewCtx, ln, leaderAddr, leaderID)
}
