// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
)

// MAX_BPF_STACK is the kernel's limit on a BPF-to-BPF call chain; maxChainStack
// keeps a frame of margin below it so growth fails here, not at load time.
const (
	maxBPFStack   = 512
	maxChainStack = 480
)

// frameRounding is the verifier's per-frame rounding with the JIT disabled;
// a JITed kernel rounds to 16, so this bound holds on either.
const frameRounding = 32

func roundFrame(depth int) int {
	if depth < 1 {
		depth = 1
	}

	return (depth + frameRounding - 1) / frameRounding * frameRounding
}

// functionFrames returns each function's stack depth, the deepest r10-relative
// access, as the verifier derives it.
func functionFrames(insns asm.Instructions) map[string]int {
	frames := make(map[string]int)
	current := ""

	for _, ins := range insns {
		if sym := ins.Symbol(); sym != "" {
			current = sym
			// A function that touches no stack is still charged a
			// rounded frame, so it must appear here.
			if _, seen := frames[current]; !seen {
				frames[current] = 0
			}
		}

		if ins.Dst != asm.R10 && ins.Src != asm.R10 {
			continue
		}

		if depth := -int(ins.Offset); depth > frames[current] {
			frames[current] = depth
		}
	}

	return frames
}

// TestStackDepthBudget bounds the combined stack of each program and its
// subprograms; summing every function over-approximates a single chain.
func TestStackDepthBudget(t *testing.T) {
	spec, err := LoadN3N6Entrypoint()
	if err != nil {
		t.Fatalf("load collection spec: %v", err)
	}

	for name, prog := range spec.Programs {
		frames := functionFrames(prog.Instructions)

		combined := 0
		for _, depth := range frames {
			combined += roundFrame(depth)
		}

		t.Logf("MEASURE %-18s combined_stack=%d frames=%v", name, combined, frames)

		if combined > maxChainStack {
			t.Errorf("%s combined stack = %d, want <= %d (kernel limit %d)",
				name, combined, maxChainStack, maxBPFStack)
		}
	}
}

// TestMeasureVerifiedInstructions logs the verifier's processed-instruction
// count per program. Numbers are kernel-specific (this box's kernel prunes
// differently than stock 5.15/6.x); use them for the relative picture, not the
// absolute fit-under-1M verdict. Run with EBPF_REQUIRE_PRIVILEGED=1.
func TestMeasureVerifiedInstructions(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 0, 1)

	for _, pr := range []struct {
		name string
		p    *ebpf.Program
	}{
		{"upf_entry", obj.UpfEntryFunc},
		{"upf_uplink", obj.UpfUplinkFunc},
		{"upf_downlink", obj.UpfDownlinkFunc},
		{"upf_gtpu_control", obj.UpfGtpuControlFunc},
	} {
		info, err := pr.p.Info()
		if err != nil {
			t.Fatalf("%s Info: %v", pr.name, err)
		}

		n, ok := info.VerifiedInstructions()
		t.Logf("MEASURE %-13s verified_insns=%d ok=%v", pr.name, n, ok)
	}
}
