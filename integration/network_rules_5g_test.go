// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"testing"

	"github.com/ellanetworks/core/integration/suites"
)

// TestIntegration5GNetworkRulesAndFlowReports runs the shared network-rule +
// flow-report suite (runNetworkRulesAndFlowReports in network_rules_common_test.go)
// over 5G PDU sessions, across IPv4 and IPv6.
func TestIntegration5GNetworkRulesAndFlowReports(t *testing.T) {
	suites.Require(t, suites.Datapath5G)

	runNetworkRulesAndFlowReports(t, "gnb")
}
