// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"os"
	"testing"
)

// TestIntegration4GHAFailover is the 4G counterpart of
// TestIntegration5GHAFailover: it brings up a 3-node Raft cluster plus a
// core-tester sidecar, attaches a UE + verifies connectivity against the
// primary core over S1AP, kills the primary, and verifies attach +
// connectivity against the surviving cluster.
//
// Shares the RAT-agnostic orchestration (cluster bring-up, leader-first
// ordering, SIGKILL sequencing, phase-marker handshake) with the 5G test via
// runHA3GPPFailover in ha_3gpp_common_test.go; only the core-tester scenario
// differs (ha/failover_connectivity_4g).
//
// Named so it does NOT match the `-run TestIntegration5GHAFailover` filter of
// the 5G HA workflow nor the `-run TestIntegration4G` filter of the srsRAN 4G
// workflow (which excludes it explicitly); it runs in its own
// integration-tests-ha-4g workflow.
func TestIntegration4GHAFailover(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runHA3GPPFailover(t, "ha/failover_connectivity_4g")
}
