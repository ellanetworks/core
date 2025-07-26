// Copyright 2024 Ella Networks

package core_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/upf/core"
)

func TestGetCPUUsagePercent(t *testing.T) {
	usage, err := core.GetCPUUsagePercent()
	if err != nil {
		t.Fatalf("Error getting CPU usage: %v", err)
	}
	if usage < 0 || usage > 100 {
		t.Fatalf("CPU usage out of bounds: %f", usage)
	}
	t.Logf("CPU Usage: %f%%", usage)
}
