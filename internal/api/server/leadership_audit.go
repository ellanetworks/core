// Copyright 2026 Ella Networks

package server

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
)

const (
	LeadershipAcquiredAction = "leadership_acquired"
	LeadershipLostAction     = "leadership_lost"
)

// LeadershipAuditCallback logs leadership transitions to the audit log.
// Implements raft.LeaderCallback.
type LeadershipAuditCallback struct {
	nodeID int
}

func NewLeadershipAuditCallback(nodeID int) *LeadershipAuditCallback {
	return &LeadershipAuditCallback{nodeID: nodeID}
}

func (c *LeadershipAuditCallback) OnBecameLeader() {
	go logger.LogAuditEvent(
		context.Background(),
		LeadershipAcquiredAction,
		"system",
		"",
		fmt.Sprintf("Node %d acquired leadership", c.nodeID),
	)
}

func (c *LeadershipAuditCallback) OnLostLeadership() {
	go logger.LogAuditEvent(
		context.Background(),
		LeadershipLostAction,
		"system",
		"",
		fmt.Sprintf("Node %d lost leadership", c.nodeID),
	)
}
