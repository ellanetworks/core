// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"github.com/ellanetworks/core/integration/suites"
	"testing"
)

// TestIntegration4GHAFailover brings up a 3-node Raft cluster plus a core-tester
// sidecar, attaches a UE and verifies connectivity against the primary over S1AP,
// kills the primary, and verifies attach + connectivity against the survivors.
//
// Its name keeps it out of the 5G HA and srsRAN 4G -run filters; it runs in the
// integration-tests-ha-4g workflow.
func TestIntegration4GHAFailover(t *testing.T) {
	suites.Require(t, suites.HA3GPP4G)

	runHA3GPPFailover(t, "ha/failover_connectivity_4g")
}
