// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import "testing"

func TestClusterTransportDisablesPipelining(t *testing.T) {
	cfg := clusterTransportConfig(nil)

	if cfg.MaxRPCsInFlight != 1 {
		t.Fatalf("MaxRPCsInFlight = %d, want 1 so a failed AppendEntries cannot stall a follower's pipeline (hashicorp/raft#612)", cfg.MaxRPCsInFlight)
	}

	if cfg.MaxPool != 3 || cfg.Timeout == 0 || cfg.Logger == nil {
		t.Fatalf("transport config lost its pool, timeout or logger: %+v", cfg)
	}
}
