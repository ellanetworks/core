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
	"golang.org/x/sys/unix"
)

// buildRecord builds a dl_buffer_map sample the way the datapath does.
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
		"length short":     binary.NativeEndian.AppendUint16(valid[:12], 0),
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
// hitting an AF_PACKET socket.
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

	for i := 0; i < maxPerQueuePackets+3; i++ {
		b.mu.Lock()
		b.enqueue(1, 1, 4, []byte{byte(i)})
		b.mu.Unlock()
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	q := b.buffers[1]
	if got := len(q.packets); got != maxPerQueuePackets {
		t.Fatalf("queue holds %d packets, want %d", got, maxPerQueuePackets)
	}

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

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1})
	b.enqueue(2, 1, 4, []byte{2})
	b.totalBytes = maxTotalBytes
	b.mu.Unlock()

	b.mu.Lock()
	b.enqueue(2, 1, 4, []byte{3})
	b.mu.Unlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.buffers[1]; ok {
		t.Error("queue of the oldest packet survived budget eviction")
	}

	q := b.buffers[2]
	if got := len(q.packets); got != 2 {
		t.Fatalf("queue holds %d packets, want 2", got)
	}

	if q.packets[1].data[0] != 3 {
		t.Error("newest packet was not admitted")
	}

	if got := q.bytes; got != 2 {
		t.Errorf("queue bytes = %d, want 2", got)
	}

	if b.totalBytes != maxTotalBytes {
		t.Errorf("totalBytes = %d, want %d", b.totalBytes, maxTotalBytes)
	}
}

func TestDropRefundsBytes(t *testing.T) {
	b, _ := newTestResponder()

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1, 2, 3})
	b.mu.Unlock()

	b.Drop(1)

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

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1})
	q := b.buffers[1]

	q.packets[0].enqueued = time.Now().Add(-queueTTL - time.Second)
	b.mu.Unlock()

	b.mu.Lock()
	b.evictExpiredLocked(time.Now())
	b.mu.Unlock()

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

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.buffers[9]; ok {
		t.Error("queue still present after Drain")
	}

	if b.totalBytes != 0 {
		t.Errorf("totalBytes = %d, want 0", b.totalBytes)
	}
}

// A packet captured before the FAR flip whose record is read only after
// the queue was popped must still be delivered.
func TestDrainDeliversLateCapturedPacket(t *testing.T) {
	b, frames := newTestResponder()

	b.mu.Lock()
	b.enqueue(3, 1, 4, []byte{1})
	b.mu.Unlock()

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

// Close may run twice on a Start() error path; the second call must not
// panic.
func TestCloseIsIdempotent(t *testing.T) {
	b, _ := newTestResponder()

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

// A Drain racing Close must not send on a closed fd, and popped packets
// must be accounted.
func TestDrainAfterCloseDoesNotSend(t *testing.T) {
	b, frames := newTestResponder()

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
}

func TestDrainConcurrentClose(t *testing.T) {
	fds := make([]int, 2)
	if err := unix.Pipe(fds); err != nil {
		t.Fatalf("pipe: %v", err)
	}

	defer func() { _ = unix.Close(fds[1]) }()

	b := NewBufferResponder(nil)
	b.injectFD = fds[0]

	firstSend := make(chan struct{}, 1)

	b.send = func(fd int, _ []byte) error {
		if b.closed {
			t.Error("send after Close")
		}

		if _, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err != nil {
			t.Errorf("send on dead fd %d: %v", fd, err)
		}

		select {
		case firstSend <- struct{}{}:
		default:
		}

		return nil
	}

	b.mu.Lock()
	for i := range maxPerQueuePackets {
		b.enqueue(1, 1, 4, []byte{byte(i)})
	}
	b.mu.Unlock()

	drained := make(chan struct{})

	go func() {
		defer close(drained)

		b.Drain(1)
	}()

	select {
	case <-firstSend:
	case <-time.After(2 * time.Second):
		t.Fatal("drain never sent")
	}

	if err := b.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case <-drained:
	case <-time.After(2 * time.Second):
		t.Fatal("drain did not terminate after Close")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.buffers[1]; ok {
		t.Error("queue still present after Drain")
	}

	if b.totalBytes != 0 {
		t.Errorf("totalBytes = %d, want 0", b.totalBytes)
	}
}

func TestDrainReinjectFailure(t *testing.T) {
	b, frames := newTestResponder()

	b.send = func(_ int, _ []byte) error {
		return errors.New("send failed")
	}

	b.mu.Lock()
	b.enqueue(1, 1, 4, []byte{1})
	b.enqueue(1, 1, 4, []byte{2})
	b.mu.Unlock()

	b.Drain(1)

	if got := len(*frames); got != 0 {
		t.Errorf("send error was not propagated: %d frames captured", got)
	}
}

func TestHandleRecordCountsMalformed(t *testing.T) {
	b, _ := newTestResponder()

	b.handleRecord(buildRecord(1, 1, 1, 4, []byte{1, 2, 3})[:10])

	if _, ok := b.buffers[1]; ok {
		t.Error("malformed record was queued")
	}
}

func TestHandleRecordQueuesValid(t *testing.T) {
	b, _ := newTestResponder()

	b.handleRecord(buildRecord(1, 1, 1, 4, []byte{1, 2, 3}))

	b.mu.Lock()
	defer b.mu.Unlock()

	if got := len(b.buffers[1].packets); got != 1 {
		t.Fatalf("queued %d packets, want 1", got)
	}
}

// The consumer must copy the payload before the next read overwrites it.
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

// A flood of concurrent enqueues must not deadlock against the consumer
// and sweeper.
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
