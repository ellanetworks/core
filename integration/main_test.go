// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/ellanetworks/core/integration/suites"
)

func TestMain(m *testing.M) {
	code := m.Run()

	if err := suites.WriteDump(); err != nil {
		fmt.Fprintf(os.Stderr, "write suite declarations: %v\n", err)
		os.Exit(1)
	}

	os.Exit(code)
}
