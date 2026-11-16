// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"errors"
	"testing"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

func TestLoadAtSessionCeiling(t *testing.T) {
	requireProgTestRun(t)

	for name, useTCX := range map[string]bool{"xdp": false, "tc": true} {
		t.Run(name, func(t *testing.T) {
			obj := NewBpfObjects(true, true, 0, 1, 0, 0)
			obj.UseTCX = useTCX
			obj.Sizing = NewMapSizing(200000)

			if err := obj.Load(); err != nil {
				var ve *ebpf.VerifierError
				if errors.As(err, &ve) {
					t.Fatalf("verifier: %+v", ve)
				}

				t.Fatalf("load: %v", err)
			}

			t.Cleanup(func() { _ = obj.Close() })

			loaded := map[string]*ebpf.Map{
				"pdrs_uplink":         obj.PdrsUplink,
				"pdrs_downlink_ip4":   obj.PdrsDownlinkIp4,
				"pdrs_downlink_ip6":   obj.PdrsDownlinkIp6,
				"sdf_classifiers":     obj.SdfClassifiers,
				"veth_tunnels":        obj.VethTunnels,
				"qer_windows":         obj.QerWindows,
				"urr_map":             obj.UrrMap,
				"framed_downlink_ip4": obj.FramedDownlinkIp4,
				"framed_downlink_ip6": obj.FramedDownlinkIp6,
				"nat_ct":              obj.NatCt,
				"flow_stats":          obj.FlowStats,
			}

			for mapName, want := range obj.Sizing.maxEntries() {
				if got := loaded[mapName].MaxEntries(); got != want {
					t.Errorf("%s max_entries = %d, want %d", mapName, got, want)
				}
			}

			fillToCapacity(t, obj.PdrsUplink, obj.Sizing.MaxSessions)

			if err := obj.LoadWithMapReplacements(); err != nil {
				t.Fatalf("reload with preserved maps at the ceiling: %v", err)
			}

			if got := countEntries(t, obj.PdrsUplink); got != int(obj.Sizing.MaxSessions) {
				t.Errorf("uplink PDRs after reload = %d, want %d", got, obj.Sizing.MaxSessions)
			}
		})
	}
}

func fillToCapacity(t *testing.T, m *ebpf.Map, capacity uint32) {
	t.Helper()

	var pdr N3N6EntrypointPdrInfo

	for teid := uint32(1); teid <= capacity; teid++ {
		if err := m.Put(teid, &pdr); err != nil {
			t.Fatalf("insert %d of %d: %v", teid, capacity, err)
		}
	}

	err := m.Put(capacity+1, &pdr)
	if !errors.Is(err, unix.E2BIG) {
		t.Fatalf("insert past capacity: %v, want E2BIG", err)
	}
}

func countEntries(t *testing.T, m *ebpf.Map) int {
	t.Helper()

	var (
		key uint32
		val N3N6EntrypointPdrInfo
		n   int
	)

	it := m.Iterate()
	for it.Next(&key, &val) {
		n++
	}

	if err := it.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}

	return n
}

func TestProjectedBytesMatchesKernelAccounting(t *testing.T) {
	requireProgTestRun(t)

	cpus, err := ebpf.PossibleCPU()
	if err != nil {
		t.Fatalf("possible CPUs: %v", err)
	}

	for _, sessions := range []uint32{DefaultMaxSessions, 50000, MaxSessionsLimit} {
		obj := NewBpfObjects(true, true, 0, 1, 0, 0)
		obj.Sizing = NewMapSizing(sessions)

		spec, err := obj.loadSpec()
		if err != nil {
			t.Fatalf("load spec: %v", err)
		}

		if err := obj.Sizing.apply(spec); err != nil {
			t.Fatalf("apply: %v", err)
		}

		if err := obj.Load(); err != nil {
			t.Fatalf("load %d sessions: %v", sessions, err)
		}

		loaded := map[string]*ebpf.Map{
			"pdrs_uplink":         obj.PdrsUplink,
			"pdrs_downlink_ip4":   obj.PdrsDownlinkIp4,
			"pdrs_downlink_ip6":   obj.PdrsDownlinkIp6,
			"sdf_classifiers":     obj.SdfClassifiers,
			"veth_tunnels":        obj.VethTunnels,
			"qer_windows":         obj.QerWindows,
			"urr_map":             obj.UrrMap,
			"framed_downlink_ip4": obj.FramedDownlinkIp4,
			"framed_downlink_ip6": obj.FramedDownlinkIp6,
			"nat_ct":              obj.NatCt,
			"flow_stats":          obj.FlowStats,
		}

		var projected, measured uint64

		for name, n := range obj.Sizing.maxEntries() {
			want := startupBytes(spec.Maps[name], n, uint64(cpus))
			got := fdinfoMemlock(loaded[name].FD())
			projected += want
			measured += got

			if want > 0 && (got < want*95/100 || got > want*105/100) {
				t.Errorf("%d sessions: %s memlock %d, projected %d", sessions, name, got, want)
			}
		}

		if total := obj.Sizing.ProjectedBytes(spec, cpus); total != projected {
			t.Errorf("%d sessions: ProjectedBytes %d, want the per-map sum %d", sessions, total, projected)
		}

		t.Logf("%d sessions: projected %d MiB, kernel %d MiB for the sized maps", sessions, projected>>20, measured>>20)

		_ = obj.Close()
	}
}
