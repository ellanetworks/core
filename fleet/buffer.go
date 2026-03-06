// Copyright 2026 Ella Networks

package fleet

import (
	"sync"

	"github.com/ellanetworks/core/fleet/client"
)

const defaultFleetBufferCapacity = 10_000

// FleetBuffer is a bounded in-memory ring buffer for flow entries destined for
// Fleet sync. Writes are non-blocking: when the buffer is full the oldest entry
// is dropped. Reads drain the entire buffer atomically.
type FleetBuffer struct {
	mu       sync.Mutex
	entries  []client.FlowEntry
	capacity int
	head     int // index of the oldest entry
	count    int // number of valid entries
}

// NewFleetBuffer creates a FleetBuffer with the given maximum capacity.
// If capacity <= 0 the default (10 000) is used.
func NewFleetBuffer(capacity int) *FleetBuffer {
	if capacity <= 0 {
		capacity = defaultFleetBufferCapacity
	}

	return &FleetBuffer{
		entries:  make([]client.FlowEntry, capacity),
		capacity: capacity,
	}
}

// EnqueueFlow appends a flow entry to the buffer. If the buffer is at
// capacity the oldest entry is silently dropped to make room.
func (b *FleetBuffer) EnqueueFlow(entry client.FlowEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()

	idx := (b.head + b.count) % b.capacity

	if b.count == b.capacity {
		// Buffer full — overwrite oldest and advance head.
		b.entries[b.head] = entry
		b.head = (b.head + 1) % b.capacity
	} else {
		b.entries[idx] = entry
		b.count++
	}
}

// DrainFlows returns all buffered flow entries and resets the buffer.
// The caller owns the returned slice. Returns nil when the buffer is empty.
func (b *FleetBuffer) DrainFlows() []client.FlowEntry {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.count == 0 {
		return nil
	}

	out := make([]client.FlowEntry, b.count)

	for i := range b.count {
		out[i] = b.entries[(b.head+i)%b.capacity]
	}

	// Reset without reallocating the backing array.
	b.head = 0
	b.count = 0

	return out
}

// Len returns the current number of buffered entries.
func (b *FleetBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.count
}
