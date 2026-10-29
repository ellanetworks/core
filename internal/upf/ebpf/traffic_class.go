// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"fmt"

	"github.com/cilium/ebpf"
)

const (
	MaxTrafficClasses = 1024
	MaxRadios         = 1024
)

type TrafficClassCounters struct {
	Packets uint64
	Bytes   uint64
}

type TrafficClassDrop struct {
	ClassID   uint16
	Direction Direction
	Reason    string
}

type RadioDrop struct {
	RadioID   uint16
	Direction Direction
	Reason    string
}

func counterKey(id uint16, capacity int, dir Direction) uint32 {
	key := uint32(int(id)%capacity) << 1
	if dir == Downlink {
		key |= 1
	}

	return key
}

func readForwarded(m *ebpf.Map, key uint32) (TrafficClassCounters, error) {
	var (
		perCPU []N3N6EntrypointTrafficClassCounters
		total  TrafficClassCounters
	)

	if err := m.Lookup(key, &perCPU); err != nil {
		return total, err
	}

	for _, c := range perCPU {
		total.Packets += c.Packets
		total.Bytes += c.Bytes
	}

	return total, nil
}

func (bpfObjects *BpfObjects) ReadTrafficClassForwarded(classID uint16, dir Direction) (TrafficClassCounters, error) {
	c, err := readForwarded(bpfObjects.TrafficClassForwarded, counterKey(classID, MaxTrafficClasses, dir))
	if err != nil {
		return c, fmt.Errorf("read traffic class %d %s counters: %w", classID, dir, err)
	}

	return c, nil
}

func (bpfObjects *BpfObjects) ReadRadioForwarded(radioID uint16, dir Direction) (TrafficClassCounters, error) {
	c, err := readForwarded(bpfObjects.RadioForwarded, counterKey(radioID, MaxRadios, dir))
	if err != nil {
		return c, fmt.Errorf("read radio %d %s counters: %w", radioID, dir, err)
	}

	return c, nil
}

func dropDirection(d uint8) Direction {
	if d == 1 {
		return Downlink
	}

	return Uplink
}

func dropReason(r uint8) string {
	names := DropReasonNames()
	if int(r) < len(names) {
		return names[r]
	}

	return "unknown"
}

func sumPerCPU(perCPU []uint64) uint64 {
	var sum uint64
	for _, v := range perCPU {
		sum += v
	}

	return sum
}

func (bpfObjects *BpfObjects) ReadTrafficClassDrops() (map[TrafficClassDrop]uint64, error) {
	var (
		key    N3N6EntrypointTrafficClassDropKey
		perCPU []uint64
	)

	out := make(map[TrafficClassDrop]uint64)
	entries := 0

	it := bpfObjects.TrafficClassDrops.Iterate()
	for it.Next(&key, &perCPU) {
		entries++
		out[TrafficClassDrop{ClassID: key.ClassId, Direction: dropDirection(key.Direction), Reason: dropReason(key.Reason)}] += sumPerCPU(perCPU)
	}

	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("iterate traffic class drops: %w", err)
	}

	bpfObjects.SetOccupancy(MapTrafficClassDrops, entries)

	return out, nil
}

func (bpfObjects *BpfObjects) ReadRadioDrops() (map[RadioDrop]uint64, error) {
	var (
		key    N3N6EntrypointRadioDropKey
		perCPU []uint64
	)

	out := make(map[RadioDrop]uint64)
	entries := 0

	it := bpfObjects.RadioDrops.Iterate()
	for it.Next(&key, &perCPU) {
		entries++
		out[RadioDrop{RadioID: key.RadioId, Direction: dropDirection(key.Direction), Reason: dropReason(key.Reason)}] += sumPerCPU(perCPU)
	}

	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("iterate radio drops: %w", err)
	}

	bpfObjects.SetOccupancy(MapRadioDrops, entries)

	return out, nil
}
