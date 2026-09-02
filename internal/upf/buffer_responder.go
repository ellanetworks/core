// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	bpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

const (
	// maxPerQueuePackets caps one session's queue.
	maxPerQueuePackets = 16

	// queueTTL matches T3513 paging retransmission: past it the UE did
	// not answer the page and the packets are stale anyway.
	queueTTL = 30 * time.Second

	// maxTotalBytes bounds every queue together, so an idle-UE flood
	// cannot pin unbounded memory.
	maxTotalBytes = 4 << 20

	// drainPace spaces re-injected packets so a drained queue is not
	// presented to the QER sliding-window rate limiter as one burst.
	drainPace = time.Millisecond
)

// queuedPacket is one captured L3 packet awaiting re-injection.
type queuedPacket struct {
	qfi      uint8
	family   uint8
	data     []byte
	enqueued time.Time
}

// packetQueue is one session's FIFO of captured packets, byte-accounted.
type packetQueue struct {
	packets []queuedPacket
	bytes   int
}

// BufferResponder consumes downlink packets the datapath captured at the
// FAR_BUFF branch and re-injects them through upf_downlink_func on a
// dedicated veth pair once the FAR flips to FORW.
type BufferResponder struct {
	bpfObjects *ebpf.BpfObjects

	reader *ringbuf.Reader
	done   chan struct{}

	evictStop chan struct{}
	evictDone chan struct{}

	vethLink link.Link

	injectFD int
	injectSA unix.SockaddrLinklayer
	closed   bool

	srcMAC [6]byte // veth-buf
	dstMAC [6]byte // veth-buf-xdp

	mu         sync.Mutex
	buffers    map[uint64]*packetQueue // keyed by local SEID
	totalBytes int

	// send injects one frame; a field so tests can capture frames
	// without an AF_PACKET socket.
	send func(fd int, frame []byte) error
}

// NewBufferResponder creates a buffer responder. It does not start the
// consumer.
func NewBufferResponder(bpfObjects *ebpf.BpfObjects) *BufferResponder {
	b := &BufferResponder{
		bpfObjects: bpfObjects,
		injectFD:   -1,
		buffers:    make(map[uint64]*packetQueue),
	}
	b.send = b.sendFrame

	return b
}

func (b *BufferResponder) sendFrame(fd int, frame []byte) error {
	return unix.Sendto(fd, frame, 0, &b.injectSA)
}

// Start creates the injection veth pair, attaches upf_downlink_func to its
// program end, opens the send-only AF_PACKET socket, and starts the
// consumer.
func (b *BufferResponder) Start() error {
	if err := createVethPair(VethBufName, VethBufXDPName); err != nil {
		return fmt.Errorf("create injection veth pair: %w", err)
	}

	xdpIdx, err := vethIndex(VethBufXDPName)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", VethBufXDPName, err)
	}

	if b.bpfObjects.UseTCX {
		b.vethLink, err = attachTCX(b.bpfObjects.UpfDownlinkFunc, xdpIdx, VethBufXDPName)
	} else {
		b.vethLink, err = attachXDP(b.bpfObjects.UpfDownlinkFunc, xdpIdx, link.XDPGenericMode)
	}

	if err != nil {
		return fmt.Errorf("attach downlink program to %s: %w", VethBufXDPName, err)
	}

	bufIface, err := net.InterfaceByName(VethBufName)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", VethBufName, err)
	}

	copy(b.srcMAC[:], bufIface.HardwareAddr)

	xdpIface, err := net.InterfaceByName(VethBufXDPName)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", VethBufXDPName, err)
	}

	copy(b.dstMAC[:], xdpIface.HardwareAddr)

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		return fmt.Errorf("AF_PACKET socket: %w", err)
	}

	b.injectFD = fd
	b.injectSA = unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  bufIface.Index,
	}

	b.reader, err = ringbuf.NewReader(b.bpfObjects.DlBufferMap)
	if err != nil {
		_ = unix.Close(fd)
		b.injectFD = -1

		return fmt.Errorf("open dl_buffer ring buffer: %w", err)
	}

	b.done = make(chan struct{})
	b.evictStop = make(chan struct{})
	b.evictDone = make(chan struct{})

	done := b.done

	go func() { // #nosec: G118 -- lifecycle goroutine
		defer close(done)

		b.consume()
	}()

	evictDone := b.evictDone

	go func() { // #nosec: G118 -- lifecycle goroutine
		defer close(evictDone)

		b.evictExpired()
	}()

	if err := b.bpfObjects.SetBufferVethIfindex(xdpIdx); err != nil {
		_ = b.Close()

		return fmt.Errorf("set buffer veth ifindex: %w", err)
	}

	logger.UpfLog.Info("downlink buffer responder started",
		zap.String("veth_go", VethBufName),
		zap.String("veth_xdp", VethBufXDPName),
		zap.Int("max_per_queue_packets", maxPerQueuePackets),
		zap.Duration("queue_ttl", queueTTL),
	)

	return nil
}

// UpdateProgram points the veth link at prog, which a reload has replaced.
func (b *BufferResponder) UpdateProgram(prog *bpf.Program) error {
	if b == nil || b.vethLink == nil {
		return nil
	}

	return b.vethLink.Update(prog)
}

// Close stops the consumer and the sweeper, then releases the socket, the
// link and the pair, in that order, so the consumer never touches a
// recycled fd. It is idempotent.
func (b *BufferResponder) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()

		return nil
	}

	b.closed = true
	b.mu.Unlock()

	// Clear the ifindex before the pair goes away: a recycled ifindex
	// must not make the NAT guard match an unrelated frame.
	if b.bpfObjects != nil {
		if err := b.bpfObjects.SetBufferVethIfindex(0); err != nil {
			logger.UpfLog.Warn("failed to clear buffer veth ifindex", zap.Error(err))
		}
	}

	if b.reader != nil {
		_ = b.reader.Close()
	}

	if b.done != nil {
		<-b.done
	}

	if b.evictStop != nil {
		close(b.evictStop)
		<-b.evictDone
	}

	b.mu.Lock()
	if b.injectFD >= 0 {
		_ = unix.Close(b.injectFD)
		b.injectFD = -1
	}
	b.mu.Unlock()

	if b.vethLink != nil {
		_ = b.vethLink.Close()
	}

	if err := destroyVethPair(VethBufName); err != nil {
		logger.UpfLog.Warn("failed to destroy injection veth pair", zap.Error(err))
	}

	return nil
}

// consume is the ring buffer consumer goroutine.
func (b *BufferResponder) consume() {
	var record ringbuf.Record

	for {
		err := b.reader.ReadInto(&record)
		if errors.Is(err, os.ErrClosed) {
			return
		}

		if err != nil {
			logger.UpfLog.Warn("dl buffer ring read error", zap.Error(err))
			continue
		}

		hdr, payload, ok := parseDlBufferRecord(record.RawSample)
		if !ok {
			incCounter(bufferRecordsMalformed)
			logger.UpfLog.Warn("malformed dl buffer record",
				zap.Int("size", len(record.RawSample)))

			continue
		}

		pkt := make([]byte, len(payload))
		copy(pkt, payload)

		b.mu.Lock()
		b.enqueue(hdr.LocalSEID, hdr.QFI, hdr.Family, pkt)
		b.mu.Unlock()
	}
}

// evictExpired drops queues older than queueTTL so a UE that never answers
// the page cannot hold memory forever.
func (b *BufferResponder) evictExpired() {
	ticker := time.NewTicker(queueTTL / 2)
	defer ticker.Stop()

	for {
		select {
		case <-b.evictStop:
			return
		case <-ticker.C:
			b.mu.Lock()
			b.evictExpiredLocked(time.Now())
			b.mu.Unlock()
		}
	}
}

// evictExpiredLocked drops every queue whose head has aged past the TTL.
// Caller holds mu.
func (b *BufferResponder) evictExpiredLocked(now time.Time) {
	for seid, q := range b.buffers {
		if len(q.packets) == 0 {
			continue
		}

		// FIFO: the head is the oldest.
		if now.Sub(q.packets[0].enqueued) < queueTTL {
			continue
		}

		addCounter(bufferPacketsEvicted, float64(len(q.packets)))
		b.totalBytes -= q.bytes
		delete(b.buffers, seid)
	}
}

// enqueue adds one captured packet under mu, applying the per-queue cap
// (head-drop, keeping the newest packets since older ones are more likely
// to have timed out from the sender's perspective) and the global byte
// budget.
func (b *BufferResponder) enqueue(seid uint64, qfi uint8, family uint8, pkt []byte) {
	q, ok := b.buffers[seid]
	if !ok {
		q = &packetQueue{}
		b.buffers[seid] = q
	}

	// Global budget first: refuse before we evict anything, so we don't
	// evict a packet and then drop the newcomer for the same reason.
	if b.totalBytes+len(pkt) > maxTotalBytes {
		incCounter(bufferPacketsEvicted)
		return
	}

	for len(q.packets) >= maxPerQueuePackets {
		incCounter(bufferPacketsEvicted)
		b.dropHead(q)
	}

	q.packets = append(q.packets, queuedPacket{
		qfi:      qfi,
		family:   family,
		data:     pkt,
		enqueued: time.Now(),
	})
	q.bytes += len(pkt)
	b.totalBytes += len(pkt)
}

// dropHead removes the oldest packet of q and refunds its bytes. Caller
// holds mu.
func (b *BufferResponder) dropHead(q *packetQueue) {
	p := q.packets[0]
	q.packets[0].data = nil
	q.packets = q.packets[1:]
	q.bytes -= len(p.data)
	b.totalBytes -= len(p.data)
}

// Drain re-injects a session's buffered packets: pop under the lock, then
// send paced and unlocked. The synthetic Ethernet header is fresh — the
// capture started at L3, so no VLAN handling exists here by construction.
func (b *BufferResponder) Drain(seid uint64) {
	b.mu.Lock()

	q, ok := b.buffers[seid]
	if !ok || len(q.packets) == 0 {
		b.mu.Unlock()
		return
	}

	packets := q.packets

	delete(b.buffers, seid)
	b.totalBytes -= q.bytes

	b.mu.Unlock()

	frame := make([]byte, 0, 14+ebpf.MaxDlBufferPkt)

	var injected int

	for _, p := range packets {
		b.mu.Lock()
		fd := b.injectFD
		closed := b.closed
		b.mu.Unlock()

		if closed || fd < 0 {
			return
		}

		frame = append(frame[:0], b.dstMAC[:]...)
		frame = append(frame, b.srcMAC[:]...)
		frame = binary.BigEndian.AppendUint16(frame, etherTypeFor(p.family))
		frame = append(frame, p.data...)

		if err := b.send(fd, frame); err != nil {
			logger.UpfLog.Warn("failed to re-inject buffered packet",
				logger.SEID(seid), zap.Error(err))
			incCounter(bufferReinjectFailed)

			continue
		}

		injected++

		time.Sleep(drainPace)
	}

	if injected > 0 {
		logger.UpfLog.Debug("drained buffered downlink packets",
			logger.SEID(seid), zap.Int("count", injected))
	}
}

// Drop discards a session's buffered packets.
func (b *BufferResponder) Drop(seid uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	q, ok := b.buffers[seid]
	if !ok {
		return
	}

	addCounter(bufferPacketsEvicted, float64(len(q.packets)))
	b.totalBytes -= q.bytes
	delete(b.buffers, seid)
}

// parseDlBufferRecord decodes a dl_buffer_map sample into its header and
// payload. A malformed record is a bug, not noise, so the caller counts it.
func parseDlBufferRecord(sample []byte) (hdr ebpf.DlBufferHeader, payload []byte, ok bool) {
	if len(sample) < 16 {
		return hdr, nil, false
	}

	hdr = ebpf.DlBufferHeader{
		LocalSEID: binary.NativeEndian.Uint64(sample[0:8]),
		PdrID:     binary.NativeEndian.Uint16(sample[8:10]),
		Len:       binary.NativeEndian.Uint16(sample[10:12]),
		QFI:       sample[12],
		Family:    sample[13],
	}

	payload = sample[16:]

	if hdr.Len == 0 || int(hdr.Len) != len(payload) || hdr.Len > ebpf.MaxDlBufferPkt {
		return hdr, nil, false
	}

	if hdr.Family != 4 && hdr.Family != 6 {
		return hdr, nil, false
	}

	return hdr, payload, true
}

func etherTypeFor(family uint8) uint16 {
	if family == 6 {
		return 0x86DD
	}

	return 0x0800
}
