// SPDX-FileCopyrightText: Ella Networks Inc.

package gmm

import (
	"testing"
)

func TestMain(m *testing.M) {
	RegisterMetrics()

	m.Run()
}
