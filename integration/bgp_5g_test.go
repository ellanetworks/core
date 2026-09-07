// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"testing"

	"github.com/ellanetworks/core/integration/suites"
)

// TestIntegration5GBGP runs the shared BGP suite (runBGPSuite in
// bgp_common_test.go) with a 5G PDU session holding the advertised UE route.
func TestIntegration5GBGP(t *testing.T) {
	suites.Require(t, suites.BGP5G)

	runBGPSuite(t, "gnb/session_hold")
}
