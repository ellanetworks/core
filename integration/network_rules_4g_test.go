// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"os"
	"testing"
)

// TestIntegration4GNetworkRules runs the shared network-rule + flow-report suite
// (runNetworkRulesAndFlowReports in network_rules_common_test.go) over 4G EPS
// bearers, asserting the UPF enforces every rule shape AND records the matching
// flow content — the same depth as the 5G test for IPv4 (8 rule shapes incl.
// uplink + port-range, with full per-flow packet/byte/5-tuple/timestamp
// assertions). The UPF rule engine and flow accounting are RAT-agnostic; the 4G
// probe (s1enb/connectivity_expect_*) drives the same protocols, port (34242),
// and payloads as the 5G one.
//
// IPv4 only: the shared TCP flow-content predicate (EachIMSIDistinctTuplesIs) is
// strict about ephemeral-tuple counts, and IPv6 drop scenarios add a retry tuple
// when the SYN-ACK is dropped. Closing the IPv6 leg needs that shared predicate
// made retry-tolerant (it affects 5G too) — tracked separately.
func TestIntegration4GNetworkRules(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	if DetectIPFamily() != IPv4Only {
		t.Skipf("TestIntegration4GNetworkRules runs in IPv4 mode, current %s", DetectIPFamily())
	}

	runNetworkRulesAndFlowReports(t, "s1enb")
}
