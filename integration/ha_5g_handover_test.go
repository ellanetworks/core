// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"testing"

	"github.com/ellanetworks/core/integration/suites"
)

func TestIntegration5GHAN2Handover(t *testing.T) {
	suites.Require(t, suites.HA3GPP5G)

	runHA3GPPHandover(t, haHandover5G, "ha/n2_handover_5g")
}

func TestIntegration5GHAXnHandover(t *testing.T) {
	suites.Require(t, suites.HA3GPP5G)

	runHA3GPPHandover(t, haHandover5G, "ha/xn_handover_5g")
}
