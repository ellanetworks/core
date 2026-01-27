// Copyright 2026 Ella Networks

package gmm

import (
	"testing"
)

func TestMain(m *testing.M) {
	RegisterMetrics()

	m.Run()
}
