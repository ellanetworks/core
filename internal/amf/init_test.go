// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/internal/metrics"
)

func TestMain(m *testing.M) {
	metrics.RegisterMetrics()

	m.Run()
}
