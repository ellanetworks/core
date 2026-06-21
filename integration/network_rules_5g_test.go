// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"os"
	"testing"
)

// TestIntegration5GNetworkRulesAndFlowReports runs the shared network-rule +
// flow-report suite (runNetworkRulesAndFlowReports in network_rules_common_test.go)
// over 5G PDU sessions, across IPv4 and IPv6.
func TestIntegration5GNetworkRulesAndFlowReports(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runNetworkRulesAndFlowReports(t, "gnb")
}
