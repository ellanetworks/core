// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"encoding/binary"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

var registerBufferMetricsOnce sync.Once

// registerBufferMetrics registers the buffer counters into the default
// registry once for this test binary. The full RegisterMetrics cannot be
// used here: its datapath collectors dereference bpfObjects, which is nil
// in tests.
func registerBufferMetrics() {
	registerBufferMetricsOnce.Do(func() {
		prometheus.MustRegister(bufferPacketsEvicted, bufferPacketsReinjected,
			bufferReinjectFailed, bufferRecordsMalformed)
	})
}

// counterValue reads one counter series from the default registry, the same
// gather-and-walk internal/metrics/metrics_test.go uses. A series that was
// never incremented is absent from the gather and reads as zero.
func counterValue(t *testing.T, name string, labels map[string]string) float64 {
	t.Helper()

	registerBufferMetrics()

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	for _, fam := range families {
		if fam.GetName() != name {
			continue
		}

		for _, m := range fam.GetMetric() {
			if labelsMatch(m.GetLabel(), labels) {
				return m.GetCounter().GetValue()
			}
		}
	}

	return 0
}

func evictedValue(t *testing.T, reason string) float64 {
	return counterValue(t, "app_upf_dl_buffer_packets_evicted_total", map[string]string{"reason": reason})
}

// labelsMatch reports whether one series carries exactly the wanted labels.
func labelsMatch(pairs []*dto.LabelPair, want map[string]string) bool {
	if len(pairs) != len(want) {
		return false
	}

	for _, p := range pairs {
		if v, ok := want[p.GetName()]; !ok || p.GetValue() != v {
			return false
		}
	}

	return true
}

// assertCounterDelta checks that a counter moved by exactly delta between
// the before and after snapshots. The counters are package globals, so tests
// assert deltas, not absolute values.
func assertCounterDelta(t *testing.T, before, after, delta float64) {
	t.Helper()

	if got := after - before; got != delta {
		t.Errorf("counter delta = %v, want %v", got, delta)
	}
}

// buildRecord builds a dl_buffer_map sample the way the datapath does:
// a 16-byte native-endian header followed by the L3 packet.
func buildRecord(seid uint64, pdrID uint16, qfi uint8, family uint8, payload []byte) []byte {
	rec := make([]byte, 16, 16+len(payload))
	binary.NativeEndian.PutUint64(rec[0:8], seid)
	binary.NativeEndian.PutUint16(rec[8:10], pdrID)
	binary.NativeEndian.PutUint16(rec[10:12], uint16(len(payload)))
	rec[12] = qfi
	rec[13] = family

	return append(rec, payload...)
}

func TestParseDlBufferRecordValid(t *testing.T) {
	payload := []byte{0x45, 0, 0, 0, 1, 2, 3, 4}

	hdr, got, ok := parseDlBufferRecord(buildRecord(42, 7, 5, 4, payload))
	if !ok {
		t.Fatal("record not parsed")
	}

	if hdr.LocalSEID != 42 || hdr.PdrID != 7 || hdr.QFI != 5 || hdr.Family != 4 {
		t.Errorf("header = %+v", hdr)
	}

	if !slices.Equal(got, payload) {
		t.Errorf("payload = %v, want %v", got, payload)
	}

	hdr6, _, ok6 := parseDlBufferRecord(buildRecord(1, 2, 3, 6, payload))
	if !ok6 || hdr6.Family != 6 {
		t.Errorf("ipv6 record not parsed: %+v ok=%v", hdr6, ok6)
	}
}

func TestParseDlBufferRecordMalformed(t *testing.T) {
	valid := buildRecord(1, 1, 1, 4, []byte{1, 2, 3})

	cases := map[string][]byte{
		"truncated header": valid[:10],
		"zero length":      buildRecord(1, 1, 1, 4, nil),
		"length short":     binary.NativeEndian.AppendUint16(valid[:12], 0), // len field says 0, payload present
		"length long": func() []byte {
			r := buildRecord(1, 1, 1, 4, []byte{1, 2, 3})
			binary.NativeEndian.PutUint16(r[10:12], 99)

			return r
		}(),
		"over max": func() []byte {
			r := buildRecord(1, 1, 1, 4, []byte{1, 2, 3})
			binary.NativeEndian.PutUint16(r[10:12], ebpf.MaxDlBufferPkt+1)

			return r
		}(),
		"bad family": func() []byte {
			r := buildRecord(1, 1, 1, 4, []byte{1, 2, 3})
			r[13] = 5

			return r
		}(),
	}

	for name, sample := range cases {
		if _, _, ok := parseDlBufferRecord(sample); ok {
			t.Errorf("%s: record accepted", name)
		}
	}
}

// newTestResponder returns a responder whose sends are captured instead of
// hitting an AF_PACKET socket, with a fake fd.
func newTestResponder() (*BufferResponder, *[][]byte) {
	b := NewBufferResponder(nil)

	frames := &[][]byte{}

	b.send = func(_ int, frame []byte) error {
		*frames = append(*frames, slices.Clone(frame))

		return nil
	}

	b.injectFD = 1

	return b, frames
}

func TestEnqueuePerQueueCapHeadDrop(t *testing.T) {
	b, _ := newTestResponder()

	evictedBefore := evictedValue(t, evictedCapHeadDrop)

	for i := 0; i < maxPerQueuePackets+3; i++ {
		b.mu.Lock()
		b.enqueue(1, 1, 4, []byte{byte(i)})
		b.mu.Unlock()
	}

	assertCounterDelta(t, evictedBefore, evictedValue(t, evictedCapHeadDrop), 3)

	b.mu.Lock()
	defer b.mu.Unlock()

	q := b.buffers[1]
	if got := len(q.packets); got != maxPerQueuePackets {
		t.Fatalf("queue holds %d packets, want %d", got, maxPerQueuePackets)
	}

	// Head-drop keeps the newest: the first three enqueued are gone.
	for i, p := range q.packets {
		if want := byte(3 + i); p.data[0] != want {
			t.Errorf("packet %d = %d, want %d", i, p.data[0], want)
		}
	}

	if b.totalBytes != q.bytes {
		t.Errorf("totalBytes %d != queue bytes %d", b.totalBytes, q.bytes)
	}
}

func TestEnqueueGlobalByteBudget(t *testing.T) {
	b, _ := newTestResponder()

	packet := make([]byte, 100)
	evictedBefore := evictedValue(t, evictedByteBudget)

	b.mu.Lock()
	b.enqueue(1, 1, 4, packet)
	b.mu.Unlock()

	b.mu.Lock()
	// Would exceed maxTotalBytes: refused, not enqueued.
	b.enqueue(2, 1, 4, make([]byte, maxTotalBytes))
	b.mu.Unlock()

	assertCounterDelta(t, evictedBefore, evictedValue(t, evictedByteBudget), 1)

	b.mu.Lock()
	defer b.mu.Unlock()

	if q := b.buffers[2]; q != nil && len(q.packets) != 0 {
		t.Errorf("over-budget packet was enqueued: %d", len(q.packets))
	}

	if got := b.buffers[1].bytes; got != len(packet) {
		t.Errorf("first queue bytes = %d, want %d", got, len(packet))
	}
}

func TestDropRefundsBytes(t *testing.T) {
	b, _ := newTestResponder()

	evictedBefore := evictedValue(t, evictedSessionDrop)

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1, 2, 3})
	b.mu.Unlock()

	b.Drop(1)

	assertCounterDelta(t, evictedBefore, evictedValue(t, evictedSessionDrop), 1)

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.buffers[1]; ok {
		t.Error("queue still present after Drop")
	}

	if b.totalBytes != 0 {
		t.Errorf("totalBytes = %d, want 0", b.totalBytes)
	}
}

func TestEvictExpiredDropsStaleQueues(t *testing.T) {
	b, _ := newTestResponder()

	evictedBefore := evictedValue(t, evictedTTLExpired)

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1})
	q := b.buffers[1]

	// Age the packet past the TTL, as the sweeper would see it.
	q.packets[0].enqueued = time.Now().Add(-queueTTL - time.Second)
	b.mu.Unlock()

	b.mu.Lock()
	b.evictExpiredLocked(time.Now())
	b.mu.Unlock()

	assertCounterDelta(t, evictedBefore, evictedValue(t, evictedTTLExpired), 1)

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.buffers[1]; ok {
		t.Error("stale queue survived eviction")
	}

	if b.totalBytes != 0 {
		t.Errorf("totalBytes = %d, want 0", b.totalBytes)
	}
}

func TestDrainFramesPacketsAndClearsQueue(t *testing.T) {
	b, frames := newTestResponder()

	reinjectedBefore := counterValue(t, "app_upf_dl_buffer_packets_reinjected_total", nil)

	ipv4 := []byte{0x45, 0, 0, 0}
	ipv6 := make([]byte, 40)

	b.mu.Lock()
	b.enqueue(9, 5, 4, ipv4)
	b.mu.Unlock()

	b.mu.Lock()
	b.enqueue(9, 5, 6, ipv6)
	b.mu.Unlock()

	b.Drain(9)

	if got := len(*frames); got != 2 {
		t.Fatalf("sent %d frames, want 2", got)
	}

	checkFrame := func(frame, payload []byte, ethertype uint16) {
		t.Helper()

		if len(frame) != 14+len(payload) {
			t.Fatalf("frame length %d, want %d", len(frame), 14+len(payload))
		}

		if got := binary.BigEndian.Uint16(frame[12:14]); got != ethertype {
			t.Errorf("ethertype = %#x, want %#x", got, ethertype)
		}

		if !slices.Equal(frame[14:], payload) {
			t.Error("payload not preserved")
		}

		if !slices.Equal(frame[0:6], b.dstMAC[:]) {
			t.Error("dst MAC not the program end of the pair")
		}

		if !slices.Equal(frame[6:12], b.srcMAC[:]) {
			t.Error("src MAC not the Go end of the pair")
		}
	}

	checkFrame((*frames)[0], ipv4, 0x0800)
	checkFrame((*frames)[1], ipv6, 0x86DD)

	assertCounterDelta(t, reinjectedBefore, counterValue(t, "app_upf_dl_buffer_packets_reinjected_total", nil), 2)

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.buffers[9]; ok {
		t.Error("queue still present after Drain")
	}

	if b.totalBytes != 0 {
		t.Errorf("totalBytes = %d, want 0", b.totalBytes)
	}
}

// A packet the datapath captured before the FAR flip, but whose ring
// buffer record the consumer reads only after the queue was first popped,
// must still be delivered: without the grace re-check it lands in a queue
// no later drain revisits and is stranded forever.
func TestDrainDeliversLateCapturedPacket(t *testing.T) {
	b, frames := newTestResponder()

	b.mu.Lock()
	b.enqueue(3, 1, 4, []byte{1})
	b.mu.Unlock()

	// The consumer lags: it enqueues the pre-flip capture shortly after
	// Drain has popped the queue and is pacing its re-injections.
	go func() {
		time.Sleep(drainPace)

		b.mu.Lock()
		b.enqueue(3, 1, 4, []byte{2})
		b.mu.Unlock()
	}()

	b.Drain(3)

	if got := len(*frames); got != 2 {
		t.Fatalf("sent %d frames, want 2", got)
	}

	if got := (*frames)[1][14]; got != 2 {
		t.Errorf("second frame payload = %d, want the late-captured packet", got)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.buffers[3]; ok {
		t.Error("queue still present after Drain")
	}
}

// Start() tears itself down on its last failure and upf.go closes again on any
// Start() error, so Close runs twice on that path. Without an idempotence
// guard the second close(b.evictStop) panics and takes the process with it.
func TestCloseIsIdempotent(t *testing.T) {
	b, _ := newTestResponder()

	// Stand in for the goroutines Start() would have launched: Close waits
	// on done and evictDone, so hand it already-finished channels.
	b.done = make(chan struct{})
	close(b.done)

	b.evictStop = make(chan struct{})
	b.evictDone = make(chan struct{})
	close(b.evictDone)

	b.injectFD = -1

	if err := b.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	if err := b.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// A Drain racing Close must not send on a closed (possibly recycled) fd,
// and the packets it had already popped must not vanish unaccounted.
func TestDrainAfterCloseDoesNotSend(t *testing.T) {
	b, frames := newTestResponder()

	evictedBefore := evictedValue(t, evictedClosed)

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1, 2, 3})
	b.mu.Unlock()

	b.mu.Lock()
	b.injectFD = -1
	b.closed = true
	b.mu.Unlock()

	b.Drain(1)

	if got := len(*frames); got != 0 {
		t.Errorf("sent %d frames after Close, want 0", got)
	}

	assertCounterDelta(t, evictedBefore, evictedValue(t, evictedClosed), 1)
}

// A Close landing mid-drain must leave the already-popped remainder
// accounted as closed, not silently discarded.
func TestDrainMidCloseDiscardsRemainder(t *testing.T) {
	b, frames := newTestResponder()

	send := b.send
	b.send = func(fd int, frame []byte) error {
		err := send(fd, frame)

		// Close lands after the first re-injection.
		b.mu.Lock()
		b.closed = true
		b.mu.Unlock()

		return err
	}

	evictedBefore := evictedValue(t, evictedClosed)
	reinjectedBefore := counterValue(t, "app_upf_dl_buffer_packets_reinjected_total", nil)

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1})
	b.enqueue(1, 1, 4, []byte{2})
	b.mu.Unlock()

	b.Drain(1)

	if got := len(*frames); got != 1 {
		t.Fatalf("sent %d frames, want 1", got)
	}

	assertCounterDelta(t, evictedBefore, evictedValue(t, evictedClosed), 1)
	assertCounterDelta(t, reinjectedBefore, counterValue(t, "app_upf_dl_buffer_packets_reinjected_total", nil), 1)
}

func TestDrainReinjectFailure(t *testing.T) {
	b, frames := newTestResponder()

	b.send = func(_ int, _ []byte) error {
		return errors.New("send failed")
	}

	failedBefore := counterValue(t, "app_upf_dl_buffer_reinject_failed_total", nil)
	reinjectedBefore := counterValue(t, "app_upf_dl_buffer_packets_reinjected_total", nil)

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1})
	b.enqueue(1, 1, 4, []byte{2})
	b.mu.Unlock()

	b.Drain(1)

	if got := len(*frames); got != 0 {
		t.Errorf("send error was not propagated: %d frames captured", got)
	}

	assertCounterDelta(t, failedBefore, counterValue(t, "app_upf_dl_buffer_reinject_failed_total", nil), 2)
	assertCounterDelta(t, reinjectedBefore, counterValue(t, "app_upf_dl_buffer_packets_reinjected_total", nil), 0)
}

func TestHandleRecordCountsMalformed(t *testing.T) {
	b, _ := newTestResponder()

	malformedBefore := counterValue(t, "app_upf_dl_buffer_records_malformed_total", nil)

	b.handleRecord(buildRecord(1, 1, 1, 4, []byte{1, 2, 3})[:10])

	assertCounterDelta(t, malformedBefore, counterValue(t, "app_upf_dl_buffer_records_malformed_total", nil), 1)

	if _, ok := b.buffers[1]; ok {
		t.Error("malformed record was queued")
	}
}

func TestHandleRecordQueuesValid(t *testing.T) {
	b, _ := newTestResponder()

	malformedBefore := counterValue(t, "app_upf_dl_buffer_records_malformed_total", nil)

	b.handleRecord(buildRecord(1, 1, 1, 4, []byte{1, 2, 3}))

	assertCounterDelta(t, malformedBefore, counterValue(t, "app_upf_dl_buffer_records_malformed_total", nil), 0)

	b.mu.Lock()
	defer b.mu.Unlock()

	if got := len(b.buffers[1].packets); got != 1 {
		t.Fatalf("queued %d packets, want 1", got)
	}
}

func TestQueuedTotals(t *testing.T) {
	b, _ := newTestResponder()

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1, 2, 3})
	b.enqueue(1, 1, 4, []byte{4, 5})
	b.enqueue(2, 1, 4, []byte{6})
	b.mu.Unlock()

	packets, bytes, sessions := b.queuedTotals()

	if packets != 3 {
		t.Errorf("packets = %d, want 3", packets)
	}

	if bytes != 6 {
		t.Errorf("bytes = %d, want 6", bytes)
	}

	if sessions != 2 {
		t.Errorf("sessions = %d, want 2", sessions)
	}
}

// RawSample is reused across ReadInto calls, so the consumer must copy the
// payload before the next read overwrites it.
func TestEnqueueCopiesPayload(t *testing.T) {
	b, _ := newTestResponder()

	sample := buildRecord(1, 1, 1, 4, []byte{1, 2, 3})

	hdr, payload, ok := parseDlBufferRecord(sample)
	if !ok {
		t.Fatal("record not parsed")
	}

	pkt := make([]byte, len(payload))
	copy(pkt, payload)

	b.mu.Lock()
	b.enqueue(hdr.LocalSEID, hdr.QFI, hdr.Family, pkt)
	b.mu.Unlock()

	// Overwrite the sample as the next ReadInto would.
	for i := range sample {
		sample[i] = 0xEE
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if got := b.buffers[1].packets[0].data; !slices.Equal(got, []byte{1, 2, 3}) {
		t.Errorf("queued packet mutated: %v", got)
	}
}

func TestEtherTypeFor(t *testing.T) {
	if etherTypeFor(4) != 0x0800 {
		t.Error("family 4 is not IPv4 ethertype")
	}

	if etherTypeFor(6) != 0x86DD {
		t.Error("family 6 is not IPv6 ethertype")
	}
}

// The consumer and the sweeper both take b.mu; a flood of concurrent
// enqueues must not deadlock against them.
func TestEnqueueConcurrent(t *testing.T) {
	b, _ := newTestResponder()

	var wg sync.WaitGroup

	for i := range 8 {
		wg.Add(1)

		go func(seid uint64) {
			defer wg.Done()

			for j := range 100 {
				b.mu.Lock()
				b.enqueue(seid, 1, 4, []byte{byte(j)})
				b.mu.Unlock()
			}
		}(uint64(i))
	}

	wg.Wait()
}
