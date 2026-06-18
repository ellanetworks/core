// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gmm

import (
	"testing"
)

func TestMain(m *testing.M) {
	RegisterMetrics()

	m.Run()
}
