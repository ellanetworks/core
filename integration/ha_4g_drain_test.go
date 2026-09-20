// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/client"
	"github.com/ellanetworks/core/integration/suites"
)

func TestIntegration4GHADrain(t *testing.T) {
	suites.Require(t, suites.HA3GPP4G)

	runHA3GPPScenario(t, "ha/drain_4g", func(ctx context.Context, leader *client.Client, nodeID client.NodeID) error {
		resp, err := leader.DrainClusterMember(ctx, nodeID)
		if err != nil {
			return fmt.Errorf("DrainClusterMember(%s): %w", nodeID, err)
		}

		if resp.DrainState != "draining" && resp.DrainState != "drained" {
			return fmt.Errorf("drainState = %q, want draining or drained", resp.DrainState)
		}

		HALogf(t, "drained node %s; drainState=%s", nodeID, resp.DrainState)

		return waitForDrained(ctx, leader, nodeID)
	})
}
