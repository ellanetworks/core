// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"os"
	"testing"

	// Side-effect import to register every scenario.
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestIntegration4GBufferedDownlink is the EPS counterpart of
// TestIntegration5GBufferedDownlink: downlink datagrams arriving while the
// receiving UE is in ECM-IDLE are delivered after the UE answers the page
// (TS 23.401 §5.3.4.3, FAR BUFF), not just subsequent ones. It additionally
// asserts that the MME pages and that the re-injected packets are plain S1-U
// G-PDUs with no PDU Session Container. The sender is another UE, so the suite
// runs with local switching enabled.
func TestIntegration4GBufferedDownlink(t *testing.T) {
	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}

	runBufferedSuite(t, "s1enb/buffered_downlink")
}
