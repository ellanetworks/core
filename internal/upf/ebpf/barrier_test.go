// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux

package ebpf

import (
	"sync"
	"testing"
)

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

func TestProgramBarrierConcurrentWaiters(t *testing.T) {
	requireProgTestRun(t)

	barrier, err := NewProgramBarrier()
	if err != nil {
		t.Fatalf("create program barrier: %v", err)
	}

	t.Cleanup(func() { _ = barrier.Close() })

	var wg sync.WaitGroup

	for range 64 {
		wg.Go(func() {
			if err := barrier.Wait(); err != nil {
				t.Errorf("wait: %v", err)
			}
		})
	}

	wg.Wait()
}

func TestProgramBarrierFailsAfterClose(t *testing.T) {
	requireProgTestRun(t)

	barrier, err := NewProgramBarrier()
	if err != nil {
		t.Fatalf("create program barrier: %v", err)
	}

	if err := barrier.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	if err := barrier.Wait(); err == nil {
		t.Fatal("wait succeeded on a closed barrier")
	}
}
