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

func TestProgramBarrierSurvivesCloseAndReload(t *testing.T) {
	requireProgTestRun(t)

	obj := NewBpfObjects(true, true, 0, 1, 0, 0)
	if err := obj.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	if err := obj.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	obj.UseTCX = true
	if err := obj.Load(); err != nil {
		t.Fatalf("reload: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	if err := obj.Programs.Wait(); err != nil {
		t.Fatalf("wait after reload: %v", err)
	}
}
