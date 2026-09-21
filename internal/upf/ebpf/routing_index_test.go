// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ebpf

import (
	"testing"

	"github.com/cilium/ebpf"
)

func TestRoutingIndexesDefaultToAttachmentIndexes(t *testing.T) {
	obj := NewBpfObjects(false, false, false, 11, 12, 0, 0)
	if obj.N3RoutingIndex != 11 || obj.N3RoutingIndex != obj.N3InterfaceIndex {
		t.Errorf("N3 routing index = %d, attachment index = %d; want 11", obj.N3RoutingIndex, obj.N3InterfaceIndex)
	}

	if obj.N6RoutingIndex != 12 || obj.N6RoutingIndex != obj.N6InterfaceIndex {
		t.Errorf("N6 routing index = %d, attachment index = %d; want 12", obj.N6RoutingIndex, obj.N6InterfaceIndex)
	}
}

func TestRoutingIndexesSharedLoader(t *testing.T) {
	for name, useTCX := range map[string]bool{"XDP": false, "TCX": true} {
		t.Run(name, func(t *testing.T) {
			obj := NewBpfObjects(false, false, false, 11, 12, 0, 0)
			obj.UseTCX = useTCX
			obj.N3RoutingIndex, obj.N6RoutingIndex = 21, 22

			spec, err := obj.loadSpec()
			if err != nil {
				t.Fatal(err)
			}

			if err := obj.loadAndAssignFromSpec(spec, &struct{}{}, nil); err != nil {
				t.Fatal(err)
			}

			for name, want := range map[string]uint32{
				"n3_ifindex": 11, "n6_ifindex": 12,
				"n3_routing_ifindex": 21, "n6_routing_ifindex": 22,
			} {
				var got uint32
				if err := spec.Variables[name].Get(&got); err != nil {
					t.Fatalf("read %s: %v", name, err)
				}

				if got != want {
					t.Errorf("%s = %d, want %d", name, got, want)
				}
			}
		})
	}
}

func TestRoutingIndexesLoadAndReload(t *testing.T) {
	requireProgTestRun(t)

	for name, useTCX := range map[string]bool{"XDP": false, "TCX": true} {
		t.Run(name, func(t *testing.T) {
			obj := NewBpfObjects(false, false, false, 11, 12, 0, 0)
			obj.UseTCX = useTCX

			t.Cleanup(func() { _ = obj.Close() })

			for i, load := range []struct {
				name string
				run  func() error
			}{
				{"Load", obj.Load},
				{"LoadWithMapReplacements", obj.LoadWithMapReplacements},
			} {
				obj.N3RoutingIndex, obj.N6RoutingIndex = uint32(21+i*10), uint32(22+i*10)

				if err := load.run(); err != nil {
					t.Fatalf("%s: %v", load.name, err)
				}

				for _, v := range []struct {
					name string
					v    *ebpf.Variable
					want uint32
				}{
					{"n3_ifindex", obj.N3Ifindex, 11},
					{"n6_ifindex", obj.N6Ifindex, 12},
					{"n3_routing_ifindex", obj.N3RoutingIfindex, uint32(21 + i*10)},
					{"n6_routing_ifindex", obj.N6RoutingIfindex, uint32(22 + i*10)},
				} {
					var got uint32
					if err := v.v.Get(&got); err != nil {
						t.Fatalf("%s: read %s: %v", load.name, v.name, err)
					}

					if got != v.want {
						t.Errorf("%s: %s = %d, want %d", load.name, v.name, got, v.want)
					}
				}
			}
		})
	}
}
