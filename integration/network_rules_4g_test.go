// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"testing"

	"github.com/ellanetworks/core/integration/suites"
)

// TestIntegration4GNetworkRules runs the shared network-rule + flow-report suite
// over 4G EPS bearers, asserting the UPF enforces every rule shape and records the
// matching flow content across IPv4 and IPv6.
func TestIntegration4GNetworkRules(t *testing.T) {
	suites.Require(t, suites.Datapath4G)

	runNetworkRulesAndFlowReports(t, "s1enb")
}
