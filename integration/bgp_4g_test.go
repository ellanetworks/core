// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"os"
	"testing"
)

// TestIntegration4GBGP runs the shared BGP suite with a 4G EPS bearer holding the
// advertised UE route, asserting the external gobgp peer receives it.
func TestIntegration4GBGP(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runBGPSuite(t, "s1enb/session_hold")
}
