// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"github.com/ellanetworks/core/integration/suites"
	"testing"
)

func TestIntegration5GUE2UE(t *testing.T) {
	suites.Require(t, suites.UE2UE)

	runUE2UESuite(t, "gnb")
}
