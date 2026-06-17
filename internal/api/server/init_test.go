// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
)

func TestMain(m *testing.M) {
	server.RegisterMetrics()

	m.Run()
}
