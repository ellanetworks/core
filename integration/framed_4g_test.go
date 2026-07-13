// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"os"
	"testing"
)

// TestIntegration4GFramedRouting attaches a 4G UE whose subscriber owns a framed
// route and asserts a host behind the UE reaches the network via the
// framed-route downlink while an off-route host does not (TS 23.501 §5.6.14).
// Runs with NAT disabled; see runFramedSuite.
func TestIntegration4GFramedRouting(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runFramedSuite(t, "s1enb")
}
