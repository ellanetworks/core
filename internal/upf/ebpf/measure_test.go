// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"testing"

	"github.com/cilium/ebpf"
)

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
	} {
		info, err := pr.p.Info()
		if err != nil {
			t.Fatalf("%s Info: %v", pr.name, err)
		}

		n, ok := info.VerifiedInstructions()
		t.Logf("MEASURE %-13s verified_insns=%d ok=%v", pr.name, n, ok)
	}
}
