// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"os"
	"testing"

	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestIntegration4GBufferedDownlink runs the s1enb buffered-downlink scenario.
// TS 23.401 §5.3.4.3
func TestIntegration4GBufferedDownlink(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runBufferedSuite(t, "s1enb/buffered_downlink")
}
