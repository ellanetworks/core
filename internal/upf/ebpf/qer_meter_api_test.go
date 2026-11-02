// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"slices"
	"testing"

	"github.com/cilium/ebpf"
)

func TestQerMeterMapSpec(t *testing.T) {
	if !slices.Contains(TrackedMaps, MapQerWindows) {
		t.Fatal("QER meters are not registered for occupancy tracking")
	}

	for name, load := range map[string]func() (*ebpf.CollectionSpec, error){
		"XDP": LoadN3N6Entrypoint,
		"TC":  LoadN3N6EntrypointTc,
	} {
		t.Run(name, func(t *testing.T) {
			spec, err := load()
			if err != nil {
				t.Fatalf("load spec: %v", err)
			}

			meter := spec.Maps[MapQerWindows]
			if meter == nil {
				t.Fatal("QER meter map is missing")
			}

			if meter.Type != ebpf.Hash {
				t.Errorf("QER meter type = %v, want Hash without eviction", meter.Type)
			}

			if want := uint32(binary.Size(N3N6EntrypointQerKey{})); meter.KeySize != want {
				t.Errorf("QER meter key size = %d, want %d", meter.KeySize, want)
			}

			if meter.ValueSize != 24 {
				t.Errorf("QER meter value size = %d, want 24 for spinlock, padding, and two windows", meter.ValueSize)
			}

			if want := spec.Maps[MapSdfClassifiers].MaxEntries * MaxSessionQERs; meter.MaxEntries != want {
				t.Errorf("QER meter capacity = %d, want %d", meter.MaxEntries, want)
			}
		})
	}
}

func TestQerMeterLifecycle(t *testing.T) {
	requireProgTestRun(t)

	spec, err := LoadN3N6Entrypoint()
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}

	meterSpec := spec.Maps[MapQerWindows].Copy()
	meterSpec.MaxEntries = 3

	meter, err := ebpf.NewMap(meterSpec)
	if err != nil {
		t.Fatalf("create QER meter map: %v", err)
	}

	t.Cleanup(func() { _ = meter.Close() })

	obj := NewBpfObjects(false, false, false, 0, 0, 0, 0)
	obj.QerWindows = meter

	checkUsage := func(want int) {
		t.Helper()

		usage, ok := obj.MapUsage()[MapQerWindows]
		if !ok || usage.Entries != want || usage.MaxEntries != meterSpec.MaxEntries {
			t.Fatalf("QER meter usage = %+v (present %t), want %d entries and capacity %d", usage, ok, want, meterSpec.MaxEntries)
		}
	}
	key := N3N6EntrypointQerKey{Seid: 0x100000001, QerId: 7}
	zero := make([]byte, meter.ValueSize())
	checkValue := func(want []byte) {
		t.Helper()

		got, err := meter.LookupBytes(key)
		if err != nil {
			t.Fatalf("read QER meter: %v", err)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("QER meter value = %x, want %x", got, want)
		}
	}

	checkUsage(0)

	if err := obj.DeleteQerMeter(key.Seid, key.QerId); err != nil {
		t.Fatalf("delete absent meter: %v", err)
	}

	checkUsage(0)

	for i, k := range []N3N6EntrypointQerKey{
		key,
		{Seid: 1, QerId: key.QerId},
		{Seid: key.Seid, QerId: key.QerId + 1},
	} {
		if err := obj.CreateQerMeter(k.Seid, k.QerId); err != nil {
			t.Fatalf("create meter %+v: %v", k, err)
		}

		checkUsage(i + 1)
	}

	checkValue(zero)

	live := bytes.Clone(zero)
	binary.NativeEndian.PutUint64(live[len(live)-16:], 123456)
	binary.NativeEndian.PutUint64(live[len(live)-8:], 654321)

	if err := meter.Update(key, live, ebpf.UpdateExist); err != nil {
		t.Fatalf("seed live meter: %v", err)
	}

	if err := obj.CreateQerMeter(key.Seid, key.QerId); !errors.Is(err, ebpf.ErrKeyExist) {
		t.Fatalf("duplicate create = %v, want ErrKeyExist", err)
	}

	checkUsage(3)
	checkValue(live)

	if err := obj.CreateQerMeter(key.Seid, key.QerId+2); err == nil {
		t.Fatal("create in full meter map succeeded")
	}

	checkUsage(3)
	checkValue(live)

	for range 2 {
		if err := obj.DeleteQerMeter(key.Seid, key.QerId); err != nil {
			t.Fatalf("delete meter: %v", err)
		}

		checkUsage(2)
	}

	if err := obj.CreateQerMeter(key.Seid, key.QerId); err != nil {
		t.Fatalf("recreate meter: %v", err)
	}

	checkUsage(3)
	checkValue(zero)
}
