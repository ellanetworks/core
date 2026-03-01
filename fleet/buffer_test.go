// Copyright 2026 Ella Networks

package fleet

import (
	"fmt"
	"sync"
	"testing"

	"github.com/ellanetworks/core/fleet/client"
)

func sampleFlow(id string) client.FlowEntry {
	return client.FlowEntry{
		SubscriberID:    id,
		SourceIP:        "10.1.0.2",
		DestinationIP:   "8.8.8.8",
		SourcePort:      44120,
		DestinationPort: 443,
		Protocol:        6,
		Packets:         142,
		Bytes:           87654,
		StartTime:       "2026-02-28T10:00:00Z",
		EndTime:         "2026-02-28T10:00:05Z",
	}
}

// ---------------------------------------------------------------------------
// NewFleetBuffer
// ---------------------------------------------------------------------------

func TestNewFleetBuffer_DefaultCapacity(t *testing.T) {
	buf := NewFleetBuffer(0)
	if buf.capacity != defaultFleetBufferCapacity {
		t.Fatalf("expected default capacity %d, got %d", defaultFleetBufferCapacity, buf.capacity)
	}
}

func TestNewFleetBuffer_CustomCapacity(t *testing.T) {
	buf := NewFleetBuffer(500)
	if buf.capacity != 500 {
		t.Fatalf("expected capacity 500, got %d", buf.capacity)
	}
}

// ---------------------------------------------------------------------------
// EnqueueFlow / DrainFlows basics
// ---------------------------------------------------------------------------

func TestFleetBuffer_DrainEmptyReturnsNil(t *testing.T) {
	buf := NewFleetBuffer(10)

	got := buf.DrainFlows()
	if got != nil {
		t.Fatalf("expected nil from empty buffer, got %d entries", len(got))
	}
}

func TestFleetBuffer_EnqueueAndDrain(t *testing.T) {
	buf := NewFleetBuffer(10)

	buf.EnqueueFlow(sampleFlow("001"))
	buf.EnqueueFlow(sampleFlow("002"))
	buf.EnqueueFlow(sampleFlow("003"))

	got := buf.DrainFlows()
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}

	if got[0].SubscriberID != "001" {
		t.Fatalf("expected first entry subscriber_id=001, got %s", got[0].SubscriberID)
	}

	if got[2].SubscriberID != "003" {
		t.Fatalf("expected third entry subscriber_id=003, got %s", got[2].SubscriberID)
	}
}

func TestFleetBuffer_DrainResetsBuffer(t *testing.T) {
	buf := NewFleetBuffer(10)

	buf.EnqueueFlow(sampleFlow("001"))

	_ = buf.DrainFlows()

	got := buf.DrainFlows()
	if got != nil {
		t.Fatalf("expected nil after second drain, got %d entries", len(got))
	}
}

func TestFleetBuffer_Len(t *testing.T) {
	buf := NewFleetBuffer(10)
	if buf.Len() != 0 {
		t.Fatalf("expected len 0, got %d", buf.Len())
	}

	buf.EnqueueFlow(sampleFlow("001"))
	buf.EnqueueFlow(sampleFlow("002"))

	if buf.Len() != 2 {
		t.Fatalf("expected len 2, got %d", buf.Len())
	}
}

// ---------------------------------------------------------------------------
// Ring buffer overflow
// ---------------------------------------------------------------------------

func TestFleetBuffer_OverflowDropsOldest(t *testing.T) {
	buf := NewFleetBuffer(3)

	buf.EnqueueFlow(sampleFlow("001"))
	buf.EnqueueFlow(sampleFlow("002"))
	buf.EnqueueFlow(sampleFlow("003"))
	// Buffer is full. Next enqueue drops "001".
	buf.EnqueueFlow(sampleFlow("004"))

	got := buf.DrainFlows()
	if len(got) != 3 {
		t.Fatalf("expected 3 entries after overflow, got %d", len(got))
	}

	if got[0].SubscriberID != "002" {
		t.Fatalf("expected oldest surviving entry to be 002, got %s", got[0].SubscriberID)
	}

	if got[2].SubscriberID != "004" {
		t.Fatalf("expected newest entry to be 004, got %s", got[2].SubscriberID)
	}
}

func TestFleetBuffer_OverflowMultipleWraps(t *testing.T) {
	buf := NewFleetBuffer(3)

	// Fill it twice over to exercise multiple wrap-arounds.
	for i := range 9 {
		buf.EnqueueFlow(sampleFlow(fmt.Sprintf("%03d", i)))
	}

	got := buf.DrainFlows()
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}

	// Should contain the last 3: 006, 007, 008.
	if got[0].SubscriberID != "006" {
		t.Fatalf("expected 006, got %s", got[0].SubscriberID)
	}

	if got[1].SubscriberID != "007" {
		t.Fatalf("expected 007, got %s", got[1].SubscriberID)
	}

	if got[2].SubscriberID != "008" {
		t.Fatalf("expected 008, got %s", got[2].SubscriberID)
	}
}

// ---------------------------------------------------------------------------
// Drain after partial refill
// ---------------------------------------------------------------------------

func TestFleetBuffer_DrainAndRefill(t *testing.T) {
	buf := NewFleetBuffer(5)

	buf.EnqueueFlow(sampleFlow("001"))
	buf.EnqueueFlow(sampleFlow("002"))

	_ = buf.DrainFlows()

	buf.EnqueueFlow(sampleFlow("003"))

	got := buf.DrainFlows()
	if len(got) != 1 {
		t.Fatalf("expected 1 entry after refill, got %d", len(got))
	}

	if got[0].SubscriberID != "003" {
		t.Fatalf("expected 003, got %s", got[0].SubscriberID)
	}
}

// ---------------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------------

func TestFleetBuffer_ConcurrentEnqueueAndDrain(t *testing.T) {
	buf := NewFleetBuffer(1000)

	var wg sync.WaitGroup

	// 10 goroutines each enqueue 100 entries.
	for g := range 10 {
		wg.Add(1)

		go func(gid int) {
			defer wg.Done()

			for i := range 100 {
				buf.EnqueueFlow(sampleFlow(fmt.Sprintf("%d-%d", gid, i)))
			}
		}(g)
	}

	wg.Wait()

	got := buf.DrainFlows()
	if len(got) != 1000 {
		t.Fatalf("expected 1000 entries, got %d", len(got))
	}
}

func TestFleetBuffer_ConcurrentEnqueueAndDrainInterleaved(t *testing.T) {
	buf := NewFleetBuffer(100)

	var wg sync.WaitGroup

	totalDrained := 0

	var mu sync.Mutex

	// Writer goroutine.

	wg.Go(func() {
		for i := range 500 {
			buf.EnqueueFlow(sampleFlow(fmt.Sprintf("%d", i)))
		}
	})

	// Drainer goroutine.

	wg.Go(func() {
		for range 50 {
			got := buf.DrainFlows()

			mu.Lock()

			totalDrained += len(got)

			mu.Unlock()
		}
	})

	wg.Wait()

	// Final drain to get whatever is left.
	got := buf.DrainFlows()
	totalDrained += len(got)

	// Due to the 100-entry buffer, some entries may be dropped. But whatever
	// was drained must be non-negative and at most 500.
	if totalDrained < 0 || totalDrained > 500 {
		t.Fatalf("unexpected total drained count: %d", totalDrained)
	}
}
