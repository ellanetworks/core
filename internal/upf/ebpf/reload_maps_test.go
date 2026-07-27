// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"reflect"
	"strings"
	"testing"
)

// TestEveryMapIsAssignedAndClassified fails if a map the BPF sources define is
// absent from N3N6EntrypointMaps, or if mapsRecreatedOnReload names a map that
// does not exist. preservedMaps derives its set from the struct, so an
// unassigned map would be silently recreated on reload and lose its state.
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
		// Internal maps (.rodata, .bss) hold the globals a reload exists to
		// change; they are not assigned and are rebuilt by design.
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

// TestPreservedMapsCoversLoadedCollection fails if a loaded map is not carried
// across a reload, or if a map is both preserved and marked for recreation.
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
