// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"os"
	"testing"
)

// TestIntegration4GBGP runs the shared BGP suite (runBGPSuite in
// bgp_common_test.go) with a 4G EPS bearer holding the advertised UE route. It
// asserts the same gold-standard signal as the 5G test: the external gobgp peer
// actually receives the 4G UE's route Ella Core advertises.
func TestIntegration4GBGP(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runBGPSuite(t, "s1enb/session_hold")
}
