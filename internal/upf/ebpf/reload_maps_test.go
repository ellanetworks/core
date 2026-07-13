// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"reflect"
	"testing"
)

// TestPreservedMapsCoversAllMaps fails if a datapath map is neither preserved
// across a reload (preservedMaps) nor explicitly exempt (mapsRecreatedOnReload),
// guarding against a newly added map losing its state on the next reload.
func TestPreservedMapsCoversAllMaps(t *testing.T) {
	var obj BpfObjects

	preserved := obj.preservedMaps()
	mapsType := reflect.TypeOf(N3N6EntrypointMaps{})

	for i := 0; i < mapsType.NumField(); i++ {
		name := mapsType.Field(i).Tag.Get("ebpf")
		if name == "" {
			continue
		}

		// profiling_map is preserved only when compiled with -DENABLE_PROFILING.
		if name == "profiling_map" {
			continue
		}

		if _, ok := preserved[name]; ok {
			continue
		}

		if mapsRecreatedOnReload[name] {
			continue
		}

		t.Errorf("map %q is neither preserved across reload nor in mapsRecreatedOnReload; "+
			"add it to preservedMaps() (or mapsRecreatedOnReload if it must be recreated), "+
			"else it loses its state on the next reload", name)
	}
}
