// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEveryMapIsAssignedAndClassified(t *testing.T) {
	spec, err := LoadN3N6Entrypoint()
	if err != nil {
		t.Fatalf("load N3/N6 spec: %v", err)
	}

	assigned := make(map[string]bool)

	mapsType := reflect.TypeOf(N3N6EntrypointMaps{})
	for i := range mapsType.NumField() {
		if name, ok := mapsType.Field(i).Tag.Lookup("ebpf"); ok {
			assigned[name] = true
		}
	}

	for name := range spec.Maps {
		if strings.HasPrefix(name, ".") {
			continue
		}

		if !assigned[name] {
			t.Errorf("map %q is defined in the BPF sources but not assigned into "+
				"N3N6EntrypointMaps, so it is recreated empty on every reload", name)
		}
	}

	for name := range mapsRecreatedOnReload {
		if _, ok := spec.Maps[name]; !ok {
			t.Errorf("mapsRecreatedOnReload names %q, which the BPF sources do not define", name)
		}
	}
}

func TestPreservedMapsCoversLoadedCollection(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgramConfig(t, false, false, 0, 1, 0, 0)

	preserved := obj.preservedMaps()

	v := reflect.ValueOf(obj.N3N6EntrypointMaps)
	mapsType := v.Type()

	for i := range mapsType.NumField() {
		name, ok := mapsType.Field(i).Tag.Lookup("ebpf")
		if !ok || mapsRecreatedOnReload[name] || v.Field(i).IsNil() {
			continue
		}

		if _, ok := preserved[name]; !ok {
			t.Errorf("map %q is loaded but not preserved across a reload, "+
				"so it loses its state when NAT or flow accounting is toggled", name)
		}
	}

	for name := range preserved {
		if mapsRecreatedOnReload[name] {
			t.Errorf("map %q is preserved but listed as recreated on reload", name)
		}
	}
}

func TestAttachedEntryProgramDispatchesUntilReloadIsReleased(t *testing.T) {
	requireProgTestRun(t)

	const (
		seid  = 0x55525306
		urrID = 26
	)

	obj := downlinkURRProgram(t, seid, urrID)
	frame := ethFrame(0x0800, ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 53, nil)))

	attached, err := obj.UpfEntryFunc.Clone()
	if err != nil {
		t.Fatalf("clone the attached entry program: %v", err)
	}

	t.Cleanup(func() { _ = attached.Close() })

	release, err := obj.ReloadPrograms()
	if err != nil {
		t.Fatalf("ReloadPrograms: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	runXDP(t, attached, frame)
	release()

	if got := downlinkURRPackets(t, obj, seid, urrID); got != 1 {
		t.Fatalf("downlink packets counted through the still-attached entry program = %d, want 1", got)
	}
}

func downlinkURRPackets(t *testing.T, obj *BpfObjects, seid uint64, urrID uint32) uint64 {
	t.Helper()

	perCPU, err := obj.ReadUrr(seid, urrID)
	if err != nil {
		t.Fatalf("ReadUrr: %v", err)
	}

	var packets uint64

	for _, v := range perCPU {
		packets += v.DownlinkPackets
	}

	return packets
}
