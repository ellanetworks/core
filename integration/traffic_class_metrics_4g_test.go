// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"testing"

	"github.com/ellanetworks/core/integration/suites"
)

func TestIntegration4GTrafficClassMetrics(t *testing.T) {
	suites.Require(t, suites.Datapath4G)

	runTrafficClassMetrics(t, "s1enb")
}
