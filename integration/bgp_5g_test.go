// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"os"
	"testing"
)

// TestIntegration5GBGP runs the shared BGP suite (runBGPSuite in
// bgp_common_test.go) with a 5G PDU session holding the advertised UE route.
func TestIntegration5GBGP(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runBGPSuite(t, "gnb/session_hold")
}
