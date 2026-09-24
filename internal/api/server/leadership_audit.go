// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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

func LeadershipAudit(nodeID string) func(ctx context.Context) {
	return func(ctx context.Context) {
		logger.LogAuditEvent(
			context.Background(),
			LeadershipAcquiredAction,
			"system",
			"",
			fmt.Sprintf("Node %s acquired leadership", nodeID),
		)

		<-ctx.Done()

		logger.LogAuditEvent(
			context.Background(),
			LeadershipLostAction,
			"system",
			"",
			fmt.Sprintf("Node %s lost leadership", nodeID),
		)
	}
}
