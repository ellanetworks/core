// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
	"golang.org/x/sys/unix"
)

func TestNATSweepAtCeiling(t *testing.T) {
	requireRoot(t)

	if testing.Short() {
		t.Skip("fills a 16Mi-entry conntrack table")
	}

	obj := ebpf.NewBpfObjects(false, true, false, 1, 0, 0, 0)
	obj.Sizing = ebpf.NewMapSizing(200000, 0, 0)

	if err := obj.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
		t.Fatal(err)
	}

	nowNs := uint64(ts.Nano())
	staleNs := nowNs - uint64((2 * natUDPRepliedTimeout).Nanoseconds())
	idleNs := nowNs - uint64((2 * natOrphanGrace).Nanoseconds())

	const (
		pairs = 4 << 20
		chunk = 1 << 16
	)

	keys := make([]ebpf.N3N6EntrypointFiveTuple, 0, 2*chunk)
	values := make([]ebpf.N3N6EntrypointNatEntry, 0, 2*chunk)

	flush := func() {
		if len(keys) == 0 {
			return
		}

		if _, err := obj.NatCt.BatchUpdate(keys, values, nil); err != nil {
			t.Fatalf("fill: %v", err)
		}

		keys, values = keys[:0], values[:0]
	}

	for i := range uint32(pairs) {
		ue := ebpf.N3N6EntrypointFiveTuple{Saddr: 0x0a000000 | i, Daddr: 0x08080808, Sport: 40000, Dport: 53, Proto: protoUDP}
		nat := ebpf.N3N6EntrypointFiveTuple{Saddr: 0x08080808, Daddr: 0xc0000000 | i, Sport: 53, Dport: 40000, Proto: protoUDP}

		refresh := idleNs
		if i%2 == 1 {
			refresh = staleNs
		}

		keys = append(keys, ue, nat)
		values = append(values,
			ebpf.N3N6EntrypointNatEntry{Peer: nat, RefreshTs: refresh, UeSide: 1, Replied: 1},
			ebpf.N3N6EntrypointNatEntry{Peer: ue, RefreshTs: refresh})

		if len(keys) == cap(keys) {
			flush()
		}
	}

	flush()

	u := &UPF{se: &engine.SessionEngine{BpfObjects: obj}}

	runtime.GC()

	var (
		peak uint64
		done = make(chan struct{})
		wg   sync.WaitGroup
	)

	wg.Go(func() {
		var ms runtime.MemStats

		for {
			runtime.ReadMemStats(&ms)
			peak = max(peak, ms.HeapInuse)

			select {
			case <-done:
				return
			case <-time.After(20 * time.Millisecond):
			}
		}
	})

	start := time.Now()
	scanned, deleted, ok := u.sweepNATConntrack()
	elapsed := time.Since(start)

	close(done)
	wg.Wait()

	t.Logf("swept %d entries, deleted %d in %v, peak heap %d MiB", scanned, deleted, elapsed, peak>>20)

	if !ok {
		t.Fatal("sweep did not complete")
	}

	if deleted != pairs {
		t.Errorf("deleted %d entries, want %d", deleted, pairs)
	}

	if peak > 128<<20 {
		t.Errorf("peak heap %d MiB during the sweep, want it bounded by the batch size", peak>>20)
	}

	remaining := 0

	var (
		k ebpf.N3N6EntrypointFiveTuple
		v ebpf.N3N6EntrypointNatEntry
	)

	it := obj.NatCt.Iterate()
	for it.Next(&k, &v) {
		if v.RefreshTs != idleNs {
			t.Fatalf("stale entry %+v survived the sweep", k)
		}

		remaining++
	}

	if remaining != pairs {
		t.Errorf("%d entries remain, want %d", remaining, pairs)
	}
}
