// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"testing"

	"github.com/ellanetworks/core/integration/suites"
)

func TestIntegration4GHAS1Handover(t *testing.T) {
	suites.Require(t, suites.HA3GPP4G)

	runHA3GPPHandover(t, haHandover4G, "ha/s1_handover_4g")
}

func TestIntegration4GHAX2Handover(t *testing.T) {
	suites.Require(t, suites.HA3GPP4G)

	runHA3GPPHandover(t, haHandover4G, "ha/x2_handover_4g")
}
