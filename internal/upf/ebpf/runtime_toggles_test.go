// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"testing"

	"github.com/cilium/ebpf"
)

func assertToggleWritable(t *testing.T, name string, v *ebpf.Variable) {
	t.Helper()

	if err := v.Set(true); err != nil {
		t.Fatalf("%s: set after load: %v", name, err)
	}

	var got bool
	if err := v.Get(&got); err != nil {
		t.Fatalf("%s: read back: %v", name, err)
	}

	if !got {
		t.Fatalf("%s: read back false after setting true", name)
	}

	if err := v.Set(false); err != nil {
		t.Fatalf("%s: reset: %v", name, err)
	}
}

func TestDatapathTogglesAreWritableAfterLoad(t *testing.T) {
	requireProgTestRun(t)

	objs := loadTCProgramConfig(t, true, 1, 1)

	assertToggleWritable(t, "local_switch", objs.LocalSwitch)
	assertToggleWritable(t, "flowact", objs.Flowact)

	if err := objs.Masquerade.Set(false); err == nil {
		t.Error("masquerade is writable after load; it is meant to stay a load-time constant")
	}
}
