// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"testing"

	"github.com/cilium/ebpf"
)

func newProfilingMap(t *testing.T, maxEntries uint32) *ebpf.Map {
	t.Helper()

	m, err := ebpf.NewMap(&ebpf.MapSpec{
		Type:       ebpf.PerCPUArray,
		KeySize:    4,
		ValueSize:  16,
		MaxEntries: maxEntries,
	})
	if err != nil {
		t.Fatalf("create profiling map: %v", err)
	}

	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Errorf("close profiling map: %v", err)
		}
	})

	return m
}

func TestReadProfilingStatsAbsentMap(t *testing.T) {
	stats, err := ReadProfilingStats(&BpfObjects{})
	if err != nil {
		t.Fatalf("profiling compiled out must not be an error: %v", err)
	}

	if stats != nil {
		t.Fatalf("stats = %v, want nil when the profiling map is absent", stats)
	}
}

func TestReadProfilingStatsHealthy(t *testing.T) {
	requireProgTestRun(t)

	stats, err := ReadProfilingStats(&BpfObjects{ProfilingMap: newProfilingMap(t, ProfNumEntries)})
	if err != nil {
		t.Fatalf("read profiling stats: %v", err)
	}

	if len(stats) != ProfNumEntries {
		t.Fatalf("len(stats) = %d, want %d", len(stats), ProfNumEntries)
	}
}

func TestReadProfilingStatsFailedReadIsNotZero(t *testing.T) {
	requireProgTestRun(t)

	stats, err := ReadProfilingStats(&BpfObjects{ProfilingMap: newProfilingMap(t, ProfNumEntries/2)})
	if err == nil {
		t.Fatal("a failed profiling map read must be reported, not published as zero")
	}

	if stats != nil {
		t.Fatalf("stats = %v, want nil so the collector publishes nothing", stats)
	}
}
