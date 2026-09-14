// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"testing"

	"github.com/ellanetworks/core/integration/suites"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestIntegration4GErrorIndication runs the s1enb error-indication scenario.
// TS 23.007 §21.7, §22
func TestIntegration4GErrorIndication(t *testing.T) {
	suites.Require(t, suites.SRSRAN4G)

	runLocalSwitchSuite(t, "s1enb/error_indication")
}
