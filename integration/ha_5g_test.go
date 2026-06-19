// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"os"
	"testing"
)

// TestIntegration5GHAFailover brings up a 3-node Raft cluster plus a
// core-tester sidecar, attaches a UE + verifies connectivity against the
// primary core over NGAP, kills the primary, and verifies registration +
// connectivity against the surviving cluster.
//
// Shares the RAT-agnostic orchestration (cluster bring-up, leader-first
// ordering, SIGKILL sequencing, phase-marker handshake) with the 4G test via
// runHA3GPPFailover in ha_3gpp_common_test.go; only the core-tester scenario
// differs (ha/failover_connectivity_5g). Runs in its own
// integration-tests-ha-5g workflow (needs both the ella-core and
// ella-core-tester images); named so it does NOT match the `-run
// TestIntegrationHA` filter of the control-plane HA workflow.
func TestIntegration5GHAFailover(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runHA3GPPFailover(t, "ha/failover_connectivity_5g")
}
