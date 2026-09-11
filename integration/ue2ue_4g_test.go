// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"testing"

	"github.com/ellanetworks/core/integration/suites"
)

func TestIntegration4GUE2UE(t *testing.T) {
	suites.Require(t, suites.UE2UE)

	runUE2UESuite(t, "s1enb")
}
