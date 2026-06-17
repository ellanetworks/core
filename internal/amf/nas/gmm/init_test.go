// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package gmm

import (
	"testing"
)

func TestMain(m *testing.M) {
	RegisterMetrics()

	m.Run()
}
