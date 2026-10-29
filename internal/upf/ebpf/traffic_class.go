// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"fmt"
)

const (
	MaxTrafficClasses = 1024
	MaxPeers          = 1024
)

type TrafficClassCounters struct {
	Packets uint64
	Bytes   uint64
}

type TrafficClassDrop struct {
	ClassID   uint16
	PeerID    uint16
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

func (bpfObjects *BpfObjects) ReadTrafficClassForwarded(classID uint16, dir Direction) (TrafficClassCounters, error) {
	var (
		perCPU []N3N6EntrypointTrafficClassCounters
		total  TrafficClassCounters
	)

	if err := bpfObjects.TrafficClassForwarded.Lookup(counterKey(classID, MaxTrafficClasses, dir), &perCPU); err != nil {
		return total, fmt.Errorf("read traffic class %d %s counters: %w", classID, dir, err)
	}

	for _, c := range perCPU {
		total.Packets += c.Packets
		total.Bytes += c.Bytes
	}

	return total, nil
}

func (bpfObjects *BpfObjects) ReadPeerForwarded(peerID uint16, dir Direction) (TrafficClassCounters, error) {
	var (
		perCPU []N3N6EntrypointTrafficClassCounters
		total  TrafficClassCounters
	)

	if err := bpfObjects.PeerForwarded.Lookup(counterKey(peerID, MaxPeers, dir), &perCPU); err != nil {
		return total, fmt.Errorf("read peer %d %s counters: %w", peerID, dir, err)
	}

	for _, c := range perCPU {
		total.Packets += c.Packets
		total.Bytes += c.Bytes
	}

	return total, nil
}

func (bpfObjects *BpfObjects) ReadTrafficClassDrops() (map[TrafficClassDrop]uint64, error) {
	var (
		key    N3N6EntrypointTrafficClassDropKey
		perCPU []uint64
	)

	names := DropReasonNames()
	out := make(map[TrafficClassDrop]uint64)
	entries := 0

	it := bpfObjects.TrafficClassDrops.Iterate()
	for it.Next(&key, &perCPU) {
		entries++

		dir := Uplink
		if key.Direction == 1 {
			dir = Downlink
		}

		reason := "unknown"
		if int(key.Reason) < len(names) {
			reason = names[key.Reason]
		}

		var sum uint64
		for _, v := range perCPU {
			sum += v
		}

		out[TrafficClassDrop{ClassID: key.ClassId, PeerID: key.PeerId, Direction: dir, Reason: reason}] += sum
	}

	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("iterate traffic class drops: %w", err)
	}

	bpfObjects.SetOccupancy(MapTrafficClassDrops, entries)

	return out, nil
}
