// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"testing"

	"github.com/ellanetworks/core/integration/suites"
)

func TestIntegration5GTrafficClassMetrics(t *testing.T) {
	suites.Require(t, suites.Datapath5G)

	runTrafficClassMetrics(t, "gnb")
}
