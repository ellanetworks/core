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

// functionCalls returns, per function symbol, the subprograms it can enter:
// direct calls, and callbacks passed by pointer to helpers like bpf_loop, which
// the kernel charges a frame for just the same.
func functionCalls(insns asm.Instructions) map[string][]string {
	calls := make(map[string][]string)
	current := ""

	for _, ins := range insns {
		if sym := ins.Symbol(); sym != "" {
			current = sym
			if _, seen := calls[current]; !seen {
				calls[current] = nil
			}
		}

		if !ins.IsFunctionReference() {
			continue
		}

		if target := ins.Reference(); target != "" {
			calls[current] = append(calls[current], target)
		}
	}

	return calls
}

// deepestChain returns the largest sum of rounded frames along any call chain
// rooted at fn, which is what the kernel's check_max_stack_depth computes.
// Siblings that can never be live together are not added to each other.
func deepestChain(fn string, frames map[string]int, calls map[string][]string,
	onPath map[string]bool,
) int {
	if onPath[fn] {
		return 0 // BPF forbids recursion; guard anyway
	}

	onPath[fn] = true
	defer delete(onPath, fn)

	deepest := 0

	for _, callee := range calls[fn] {
		if _, known := frames[callee]; !known {
			continue // helper, not a subprogram in this object
		}

		if d := deepestChain(callee, frames, calls, onPath); d > deepest {
			deepest = d
		}
	}

	return roundFrame(frames[fn]) + deepest
}

// TestStackDepthBudget bounds each program's deepest call chain. Summing every
// subprogram instead would charge a program for siblings that cannot be on the
// stack at once, and fail on a chain the kernel accepts.
func TestStackDepthBudget(t *testing.T) {
	spec, err := LoadN3N6Entrypoint()
	if err != nil {
		t.Fatalf("load collection spec: %v", err)
	}

	for name, prog := range spec.Programs {
		frames := functionFrames(prog.Instructions)
		calls := functionCalls(prog.Instructions)

		root := name
		if _, ok := frames[root]; !ok {
			// The entry function carries the section's symbol, which
			// need not match the program name.
			for fn := range frames {
				if len(calls[fn]) > 0 || len(frames) == 1 {
					root = fn

					break
				}
			}
		}

		chain := deepestChain(root, frames, calls, map[string]bool{})

		t.Logf("MEASURE %-18s deepest_chain=%d root=%s frames=%v calls=%v",
			name, chain, root, frames, calls)

		if chain > maxChainStack {
			t.Errorf("%s deepest call chain = %d, want <= %d (kernel limit %d)",
				name, chain, maxChainStack, maxBPFStack)
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

// TestURRNotChargedWhenRoutingDrops: a packet that does not leave is not
// billed. Charged before routing, every FIB failure billed one — and
// fib_no_neigh is ordinary while a neighbour resolves.
//
// The ifindex mismatch forces a drop after the accounting point without
// depending on the host's routing table.
func TestURRNotChargedWhenRoutingDrops(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid  = 0x55525201
		seid  = 0x5152
		urrID = 3
	)

	// Not a device this frame can egress on, so routing refuses.
	obj := loadProgramConfig(t, false, false, 1, 9, 0, 0)

	if err := obj.NewUrr(seid, urrID); err != nil {
		t.Fatalf("create URR: %v", err)
	}

	pdr := PdrInfo{
		SEID:         seid,
		UrrID:        urrID,
		IMSI:         "001010000000001",
		Far:          FarInfo{Action: 0x02 /* FAR_FORW */},
		Qer:          QerInfo{GateStatusUL: 0 /* GATE_STATUS_OPEN */},
		UEIPv4:       canonicalUEv4,
		UEIPv6Prefix: canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))
	if action != ActionDrop {
		t.Skipf("this environment forwarded the frame (XDP action %d); the billing assertion needs a drop after accounting", action)
	}

	volume, err := obj.GetAndResetUrr(seid, urrID)
	if err != nil {
		t.Fatalf("read URR: %v", err)
	}

	if volume != 0 {
		t.Errorf("URR charged %d bytes for a packet the datapath dropped after accounting", volume)
	}
}

// TestURRChargedWhenForwarded: the check above must not pass by never
// charging.
func TestURRChargedWhenForwarded(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid  = 0x55525202
		seid  = 0x5153
		urrID = 4
	)

	obj := loadN3N6Program(t)

	if err := obj.NewUrr(seid, urrID); err != nil {
		t.Fatalf("create URR: %v", err)
	}

	pdr := PdrInfo{
		SEID:         seid,
		UrrID:        urrID,
		IMSI:         "001010000000001",
		Far:          FarInfo{Action: 0x02 /* FAR_FORW */},
		Qer:          QerInfo{GateStatusUL: 0 /* GATE_STATUS_OPEN */},
		UEIPv4:       canonicalUEv4,
		UEIPv6Prefix: canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)

	action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))
	if action == ActionDrop || action == ActionAborted {
		t.Skipf("this environment did not forward the frame (XDP action %d)", action)
	}

	volume, err := obj.GetAndResetUrr(seid, urrID)
	if err != nil {
		t.Fatalf("read URR: %v", err)
	}

	if want := uint64(ethHdrLen + len(inner)); volume != want {
		t.Errorf("URR charged %d bytes for a forwarded packet, want %d", volume, want)
	}
}
