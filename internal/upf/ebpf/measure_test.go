// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
)

const (
	maxBPFStack   = 512
	maxChainStack = 480
)

const frameRounding = 32

func roundFrame(depth int) int {
	if depth < 1 {
		depth = 1
	}

	return (depth + frameRounding - 1) / frameRounding * frameRounding
}

func functionFrames(insns asm.Instructions) map[string]int {
	frames := make(map[string]int)
	current := ""

	for _, ins := range insns {
		if sym := ins.Symbol(); sym != "" {
			current = sym
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

func deepestChain(fn string, frames map[string]int, calls map[string][]string,
	onPath map[string]bool,
) int {
	if onPath[fn] {
		return 0
	}

	onPath[fn] = true
	defer delete(onPath, fn)

	deepest := 0

	for _, callee := range calls[fn] {
		if _, known := frames[callee]; !known {
			continue
		}

		if d := deepestChain(callee, frames, calls, onPath); d > deepest {
			deepest = d
		}
	}

	return roundFrame(frames[fn]) + deepest
}

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

func TestURRNotChargedWhenRoutingDrops(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid  = 0x55525201
		seid  = 0x5152
		urrID = 3
	)

	obj := loadProgramConfig(t, false, false, 1, 9, 0, 0)

	if err := obj.NewUrr(seid, urrID); err != nil {
		t.Fatalf("create URR: %v", err)
	}

	pdr := PdrInfo{
		SEID:         seid,
		UrrID:        urrID,
		IMSI:         "001010000000001",
		Far:          FarInfo{Action: 0x02},
		Qer:          QerInfo{GateStatusUL: 0},
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
		Far:          FarInfo{Action: 0x02},
		Qer:          QerInfo{GateStatusUL: 0},
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
