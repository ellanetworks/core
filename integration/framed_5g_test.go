// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"testing"

	"github.com/ellanetworks/core/integration/suites"
)

// TestIntegration5GFramedRouting establishes a 5G PDU session whose subscriber
// owns a framed route and asserts a host behind the UE reaches the network via
// the framed-route downlink while an off-route host does not (TS 23.501
// §5.6.14). Runs with NAT disabled; see runFramedSuite.
func TestIntegration5GFramedRouting(t *testing.T) {
	suites.Require(t, suites.Framed)

	runFramedSuite(t, "gnb")
}

// TestIntegration5GFramedRoutingReconcile asserts that adding or removing a
// framed route on a live PDU session releases it with cause #39 "reactivation
// requested" so the UE re-establishes with the new routes (TS 23.501 §5.6.14).
func TestIntegration5GFramedRoutingReconcile(t *testing.T) {
	suites.Require(t, suites.Framed)

	runFramedReconcileSuite(t, "gnb/framed_route_add_live", "gnb/framed_route_remove_live")
}
