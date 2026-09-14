// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"testing"

	"github.com/ellanetworks/core/integration/suites"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

// TestIntegration5GErrorIndication runs the gnb error-indication scenario.
// TS 23.527 §5.3.2
func TestIntegration5GErrorIndication(t *testing.T) {
	suites.Require(t, suites.Datapath5G)

	runLocalSwitchSuite(t, "gnb/error_indication")
}
