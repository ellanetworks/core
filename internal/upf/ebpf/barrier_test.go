// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package ebpf

import "testing"

func TestProgramBarrierWaits(t *testing.T) {
	requireProgTestRun(t)

	barrier, err := NewProgramBarrier()
	if err != nil {
		t.Fatalf("create program barrier: %v", err)
	}

	t.Cleanup(func() { _ = barrier.Close() })

	for range 3 {
		if err := barrier.Wait(); err != nil {
			t.Fatalf("wait: %v", err)
		}
	}
}
