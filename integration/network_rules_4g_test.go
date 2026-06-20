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
// flow content — the same depth and IP-family coverage as the 5G test. The UPF
// rule engine and flow accounting are RAT-agnostic; the 4G probe
// (s1enb/connectivity_expect_*[_ipv6]) drives the same protocols, port (34242),
// and payloads as the 5G one across IPv4 and IPv6.
func TestIntegration4GNetworkRules(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runNetworkRulesAndFlowReports(t, "s1enb")
}
