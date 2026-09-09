// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"errors"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
)

func evictionCount(t *testing.T, reason string) float64 {
	t.Helper()

	var m dto.Metric
	if err := dlBufferEvicted.WithLabelValues(reason).Write(&m); err != nil {
		t.Fatalf("read %s counter: %v", reason, err)
	}

	return m.GetCounter().GetValue()
}

func evictionDelta(t *testing.T, reason string, act func()) float64 {
	t.Helper()

	before := evictionCount(t, reason)

	act()

	return evictionCount(t, reason) - before
}

func TestDlBufferEvictedTTL(t *testing.T) {
	b, _ := newTestResponder()

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1})
	b.enqueue(1, 1, 4, []byte{2})
	b.buffers[1].packets[0].enqueued = time.Now().Add(-queueTTL - time.Second)
	b.mu.Unlock()

	got := evictionDelta(t, dlBufferEvictTTL, func() {
		b.mu.Lock()
		b.evictExpiredLocked(time.Now())
		b.mu.Unlock()
	})

	if got != 2 {
		t.Errorf("ttl evictions = %v, want 2", got)
	}
}

func TestDlBufferEvictedQueueDepth(t *testing.T) {
	b, _ := newTestResponder()

	got := evictionDelta(t, dlBufferEvictQueueDepth, func() {
		for i := 0; i < maxPerQueuePackets+3; i++ {
			b.mu.Lock()
			b.enqueue(1, 1, 4, []byte{byte(i)})
			b.mu.Unlock()
		}
	})

	if got != 3 {
		t.Errorf("queue_depth evictions = %v, want 3", got)
	}
}

func TestDlBufferEvictedByteBudget(t *testing.T) {
	b, _ := newTestResponder()

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1})
	b.totalBytes = maxTotalBytes
	b.mu.Unlock()

	got := evictionDelta(t, dlBufferEvictByteBudget, func() {
		b.mu.Lock()
		b.enqueue(2, 1, 4, []byte{2})
		b.mu.Unlock()
	})

	if got != 1 {
		t.Errorf("byte_budget evictions = %v, want 1", got)
	}
}

func TestDlBufferEvictedSessionDrop(t *testing.T) {
	b, _ := newTestResponder()

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1})
	b.enqueue(1, 1, 4, []byte{2})
	b.mu.Unlock()

	got := evictionDelta(t, dlBufferEvictSessionDrop, func() {
		b.Drop(1)
		b.Drop(1)
	})

	if got != 2 {
		t.Errorf("session_drop evictions = %v, want 2", got)
	}
}

func TestDlBufferEvictedMalformed(t *testing.T) {
	b, _ := newTestResponder()

	got := evictionDelta(t, dlBufferEvictMalformed, func() {
		b.handleRecord(buildRecord(1, 1, 1, 4, []byte{1, 2, 3})[:10])
		b.handleRecord(buildRecord(1, 1, 1, 4, []byte{1, 2, 3}))
	})

	if got != 1 {
		t.Errorf("malformed evictions = %v, want 1", got)
	}
}

func TestDlBufferEvictedReinjectFailed(t *testing.T) {
	b, _ := newTestResponder()

	b.send = func(_ int, _ []byte) error {
		return errors.New("send failed")
	}

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1})
	b.enqueue(1, 1, 4, []byte{2})
	b.mu.Unlock()

	got := evictionDelta(t, dlBufferEvictReinjectFailed, func() {
		b.Drain(1)
	})

	if got != 2 {
		t.Errorf("reinject_failed evictions = %v, want 2", got)
	}
}
