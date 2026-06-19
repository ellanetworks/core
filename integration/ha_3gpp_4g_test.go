// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"os"
	"testing"
)

// TestIntegration4GHAFailover is the 4G counterpart of
// TestIntegration3GPPHAFailover: it brings up a 3-node Raft cluster plus a
// core-tester sidecar, attaches a UE + verifies connectivity against the
// primary core over S1AP, kills the primary, and verifies attach +
// connectivity against the surviving cluster.
//
// Shares the RAT-agnostic orchestration (cluster bring-up, leader-first
// ordering, SIGKILL sequencing, phase-marker handshake) with the 5G test via
// runHA3GPPFailover; only the core-tester scenario differs.
//
// Named so it does NOT match the `-run TestIntegration3GPPHAFailover` filter of
// the 5G HA3GPP workflow nor the `-run TestIntegration4G` filter of the srsRAN
// 4G workflow (which excludes it explicitly); it runs in its own
// integration-tests-ha3gpp-4g workflow.
func TestIntegration4GHAFailover(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runHA3GPPFailover(t, "s1enb/failover_connectivity")
}
