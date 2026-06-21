// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"os"
	"testing"
)

// TestIntegration4GNetworkRules runs the shared network-rule + flow-report suite
// over 4G EPS bearers, asserting the UPF enforces every rule shape and records the
// matching flow content across IPv4 and IPv6.
func TestIntegration4GNetworkRules(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runNetworkRulesAndFlowReports(t, "s1enb")
}
