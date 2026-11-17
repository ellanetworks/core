// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"

	bpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/ringbuf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

const (
	VethReasmName    = "veth-reasm"
	VethReasmXDPName = "veth-reasm-xdp"

	gtpuPort        = 2152
	gtpuMsgGPDU     = 0xff
	gtpuMsgErrorInd = 26
	gtpuIETEIDData  = 16
	gtpuIEPeerAddr  = 133
	gtpuMinHeader   = 8
	ethHeaderLen    = 14
	ipv4HeaderLen   = 20
	ipv6HeaderLen   = 40
	udpHeaderLen    = 8
	reinjectTTL     = 64
	ipProtoUDP      = 17
	etherTypeIPv4   = 0x0800
	etherTypeIPv6   = 0x86DD
	maxReinjectedIP = vethMTU
)

const (
	directionUplink   = "uplink"
	directionDownlink = "downlink"

	reasmReassembled  = "reassembled"
	reasmIgnored      = "ignored"
	reasmUnknownTEID  = "unknown_teid"
	reasmTooLarge     = "too_large"
	reasmInjectFailed = "inject_failed"
	fragTxSent        = "sent"
	fragTxSendFailed  = "send_failed"
	fragTxMalformed   = "malformed"
)

var gtpuFragmentation = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "app_upf_gtpu_fragmentation_total",
	Help: "GTP-U packets the radio fragmented (uplink) and GTP-U packets sent for fragmentation (downlink), by direction and result.",
}, []string{"direction", "result"})

type gtpuSender interface {
	ServeGTPU(receiver engine.GTPUReceiver) error
	SendGTPU(local netip.Addr, peer netip.Addr, tclass uint8, payload []byte) error
}

type FragmentationResponder struct {
	bpfObjects *ebpf.BpfObjects
	gtpu       gtpuSender
	n3Ifindex  map[int]bool
	knownTEID  func(teid uint32) bool

	vethLink link.Link
	reader   *ringbuf.Reader
	done     chan struct{}

	mu       sync.Mutex
	injectFD int
	injectSA unix.SockaddrLinklayer
	closed   bool

	srcMAC [6]byte
	dstMAC [6]byte

	send func(fd int, frame []byte) error
}

func NewFragmentationResponder(bpfObjects *ebpf.BpfObjects, gtpu gtpuSender, knownTEID func(teid uint32) bool, n3Ifindexes ...int) *FragmentationResponder {
	allowed := make(map[int]bool, len(n3Ifindexes))

	for _, idx := range n3Ifindexes {
		if idx > 0 {
			allowed[idx] = true
		}
	}

	return &FragmentationResponder{
		bpfObjects: bpfObjects,
		gtpu:       gtpu,
		n3Ifindex:  allowed,
		knownTEID:  knownTEID,
		injectFD:   -1,
		send: func(fd int, frame []byte) error {
			return errors.New("not started")
		},
	}
}

func (f *FragmentationResponder) Start() error {
	if err := createVethPair(VethReasmName, VethReasmXDPName); err != nil {
		return fmt.Errorf("create reassembly veth pair: %w", err)
	}

	xdpIdx, err := vethIndex(VethReasmXDPName)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", VethReasmXDPName, err)
	}

	if f.bpfObjects.UseTCX {
		f.vethLink, err = attachTCX(f.bpfObjects.UpfUplinkFunc, xdpIdx, VethReasmXDPName)
	} else {
		f.vethLink, err = attachXDP(f.bpfObjects.UpfUplinkFunc, xdpIdx, link.XDPGenericMode)
	}

	if err != nil {
		return fmt.Errorf("attach uplink program to %s: %w", VethReasmXDPName, err)
	}

	goIface, err := net.InterfaceByName(VethReasmName)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", VethReasmName, err)
	}

	xdpIface, err := net.InterfaceByName(VethReasmXDPName)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", VethReasmXDPName, err)
	}

	copy(f.srcMAC[:], goIface.HardwareAddr)
	copy(f.dstMAC[:], xdpIface.HardwareAddr)

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, 0)
	if err != nil {
		return fmt.Errorf("AF_PACKET socket: %w", err)
	}

	f.mu.Lock()
	f.injectFD = fd
	f.injectSA = unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  goIface.Index,
	}
	f.send = func(fd int, frame []byte) error {
		sa := f.injectSA
		sa.Protocol = htons(binary.BigEndian.Uint16(frame[12:14]))

		return unix.Sendto(fd, frame, 0, &sa)
	}
	f.mu.Unlock()

	f.reader, err = ringbuf.NewReader(f.bpfObjects.FragTxMap)
	if err != nil {
		return fmt.Errorf("open frag_tx ring buffer: %w", err)
	}

	f.done = make(chan struct{})

	done := f.done

	go func() { // #nosec: G118 -- lifecycle goroutine
		defer close(done)

		f.consume()
	}()

	if err := f.gtpu.ServeGTPU(f.receive); err != nil {
		return fmt.Errorf("open the N3 GTP-U sockets: %w", err)
	}

	if err := f.bpfObjects.SetReasmVethIfindex(xdpIdx); err != nil {
		return fmt.Errorf("set reassembly veth ifindex: %w", err)
	}

	if err := f.bpfObjects.SetFragTxEnabled(true); err != nil {
		return fmt.Errorf("enable downlink outer fragmentation: %w", err)
	}

	logger.UpfLog.Info("GTP-U fragmentation responder started",
		zap.String("veth_go", VethReasmName),
		zap.String("veth_xdp", VethReasmXDPName),
	)

	return nil
}

func (f *FragmentationResponder) UpdateProgram(prog *bpf.Program) error {
	if f == nil {
		return nil
	}

	f.mu.Lock()
	l := f.vethLink
	f.mu.Unlock()

	if l == nil {
		return nil
	}

	return l.Update(prog)
}

func (f *FragmentationResponder) detach() error {
	f.mu.Lock()
	l := f.vethLink
	f.vethLink = nil
	f.mu.Unlock()

	if l == nil {
		return nil
	}

	return l.Close()
}

func (f *FragmentationResponder) Close() error {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()

		return nil
	}

	f.closed = true
	f.mu.Unlock()

	if f.bpfObjects != nil {
		if err := f.bpfObjects.SetFragTxEnabled(false); err != nil {
			logger.UpfLog.Warn("failed to disable downlink outer fragmentation", zap.Error(err))
		}

		if err := f.bpfObjects.SetReasmVethIfindex(0); err != nil {
			logger.UpfLog.Warn("failed to clear reassembly veth ifindex", zap.Error(err))
		}
	}

	if f.gtpu != nil {
		_ = f.gtpu.ServeGTPU(nil)
	}

	if f.reader != nil {
		_ = f.reader.Close()
	}

	if f.done != nil {
		<-f.done
	}

	f.mu.Lock()
	if f.injectFD >= 0 {
		_ = unix.Close(f.injectFD)
		f.injectFD = -1
	}
	f.mu.Unlock()

	_ = f.detach()

	if err := destroyVethPair(VethReasmName); err != nil {
		logger.UpfLog.Warn("failed to destroy reassembly veth pair", zap.Error(err))
	}

	return nil
}

func (f *FragmentationResponder) receive(d engine.GTPUDatagram) {
	if !d.Reassembled {
		gtpuFragmentation.WithLabelValues(directionUplink, reasmIgnored).Inc()
		return
	}

	if len(f.n3Ifindex) > 0 && !f.n3Ifindex[d.IfIndex] {
		gtpuFragmentation.WithLabelValues(directionUplink, reasmIgnored).Inc()
		return
	}

	if len(d.Payload) < gtpuMinHeader || d.Payload[1] != gtpuMsgGPDU {
		gtpuFragmentation.WithLabelValues(directionUplink, reasmIgnored).Inc()
		return
	}

	if teid := binary.BigEndian.Uint32(d.Payload[4:8]); teid != 0 && f.knownTEID != nil && !f.knownTEID(teid) {
		gtpuFragmentation.WithLabelValues(directionUplink, reasmUnknownTEID).Inc()
		f.sendErrorIndication(d, teid)

		return
	}

	frame, ok := reinjectionFrame(f.dstMAC, f.srcMAC, d)
	if !ok {
		gtpuFragmentation.WithLabelValues(directionUplink, reasmTooLarge).Inc()
		return
	}

	f.mu.Lock()
	fd := f.injectFD
	closed := f.closed

	var err error
	if closed || fd < 0 {
		err = errors.New("responder closed")
	} else {
		err = f.send(fd, frame)
	}
	f.mu.Unlock()

	if err != nil {
		gtpuFragmentation.WithLabelValues(directionUplink, reasmInjectFailed).Inc()
		logger.UpfLog.Debug("failed to re-inject a reassembled GTP-U datagram",
			zap.String("peer", d.Peer.String()), zap.Error(err))

		return
	}

	gtpuFragmentation.WithLabelValues(directionUplink, reasmReassembled).Inc()
}

func (f *FragmentationResponder) sendErrorIndication(d engine.GTPUDatagram, teid uint32) {
	if f.gtpu == nil {
		return
	}

	if err := f.gtpu.SendGTPU(d.Local.Addr(), d.Peer.Addr(), 0, errorIndicationPDU(teid, d.Local.Addr())); err != nil {
		logger.UpfLog.Debug("failed to send a GTP-U Error Indication",
			zap.String("peer", d.Peer.String()), logger.TEID(teid), zap.Error(err))
	}
}

func errorIndicationPDU(teid uint32, local netip.Addr) []byte {
	addr := local.AsSlice()
	ies := 5 + 3 + len(addr)

	pdu := make([]byte, 12+ies)
	pdu[0] = 0x32
	pdu[1] = gtpuMsgErrorInd
	binary.BigEndian.PutUint16(pdu[2:4], uint16(4+ies))
	pdu[12] = gtpuIETEIDData
	binary.BigEndian.PutUint32(pdu[13:17], teid)
	pdu[17] = gtpuIEPeerAddr
	binary.BigEndian.PutUint16(pdu[18:20], uint16(len(addr)))
	copy(pdu[20:], addr)

	return pdu
}

func reinjectionFrame(dstMAC, srcMAC [6]byte, d engine.GTPUDatagram) ([]byte, bool) {
	src := d.Peer.Addr()
	dst := d.Local.Addr()

	if src.Is4() != dst.Is4() {
		return nil, false
	}

	ipLen := ipv6HeaderLen
	etherType := uint16(etherTypeIPv6)

	if src.Is4() {
		ipLen = ipv4HeaderLen
		etherType = etherTypeIPv4
	}

	udpLen := udpHeaderLen + len(d.Payload)
	if ipLen+udpLen > maxReinjectedIP {
		return nil, false
	}

	frame := make([]byte, ethHeaderLen+ipLen+udpLen)

	copy(frame[0:6], dstMAC[:])
	copy(frame[6:12], srcMAC[:])
	binary.BigEndian.PutUint16(frame[12:14], etherType)

	ip := frame[ethHeaderLen : ethHeaderLen+ipLen]
	udp := frame[ethHeaderLen+ipLen:]

	binary.BigEndian.PutUint16(udp[0:2], d.Peer.Port())
	binary.BigEndian.PutUint16(udp[2:4], d.Local.Port())
	binary.BigEndian.PutUint16(udp[4:6], uint16(udpLen))
	copy(udp[udpHeaderLen:], d.Payload)

	if src.Is4() {
		ip[0] = 0x45
		binary.BigEndian.PutUint16(ip[2:4], uint16(ipLen+udpLen))
		ip[8] = reinjectTTL
		ip[9] = ipProtoUDP
		s4, d4 := src.As4(), dst.As4()
		copy(ip[12:16], s4[:])
		copy(ip[16:20], d4[:])
		binary.BigEndian.PutUint16(ip[10:12], ^onesComplementSum(0, ip))

		return frame, true
	}

	ip[0] = 0x60
	binary.BigEndian.PutUint16(ip[4:6], uint16(udpLen))
	ip[6] = ipProtoUDP
	ip[7] = reinjectTTL
	s6, d6 := src.As16(), dst.As16()
	copy(ip[8:24], s6[:])
	copy(ip[24:40], d6[:])

	sum := onesComplementSum(0, ip[8:40])
	sum = onesComplementSum(sum, []byte{0, 0, byte(udpLen >> 8), byte(udpLen), 0, 0, 0, ipProtoUDP})
	sum = onesComplementSum(sum, udp)

	csum := ^sum
	if csum == 0 {
		csum = 0xffff
	}

	binary.BigEndian.PutUint16(udp[6:8], csum)

	return frame, true
}

func onesComplementSum(initial uint16, b []byte) uint16 {
	sum := uint32(initial)

	for i := 0; i+1 < len(b); i += 2 {
		sum += uint32(binary.BigEndian.Uint16(b[i : i+2]))
	}

	if len(b)%2 == 1 {
		sum += uint32(b[len(b)-1]) << 8
	}

	for sum > 0xffff {
		sum = (sum & 0xffff) + (sum >> 16)
	}

	return uint16(sum)
}

func (f *FragmentationResponder) consume() {
	var record ringbuf.Record

	for {
		err := f.reader.ReadInto(&record)
		if errors.Is(err, os.ErrClosed) {
			return
		}

		if err != nil {
			logger.UpfLog.Warn("frag_tx ring read error", zap.Error(err))
			continue
		}

		f.transmit(record.RawSample)
	}
}

func (f *FragmentationResponder) transmit(pkt []byte) {
	local, peer, tclass, payload, ok := parseEncapsulatedPacket(pkt)
	if !ok {
		gtpuFragmentation.WithLabelValues(directionDownlink, fragTxMalformed).Inc()
		return
	}

	if err := f.gtpu.SendGTPU(local, peer, tclass, payload); err != nil {
		gtpuFragmentation.WithLabelValues(directionDownlink, fragTxSendFailed).Inc()
		logger.UpfLog.Debug("failed to send a downlink GTP-U packet for fragmentation",
			zap.String("peer", peer.String()), zap.Error(err))

		return
	}

	gtpuFragmentation.WithLabelValues(directionDownlink, fragTxSent).Inc()
}

func parseEncapsulatedPacket(pkt []byte) (local, peer netip.Addr, tclass uint8, payload []byte, ok bool) {
	if len(pkt) < ipv4HeaderLen {
		return local, peer, 0, nil, false
	}

	var (
		udp   []byte
		total int
	)

	switch pkt[0] >> 4 {
	case 4:
		ihl := int(pkt[0]&0x0f) * 4
		total = int(binary.BigEndian.Uint16(pkt[2:4]))

		if ihl < ipv4HeaderLen || pkt[9] != ipProtoUDP || total > len(pkt) || total < ihl+udpHeaderLen {
			return local, peer, 0, nil, false
		}

		tclass = pkt[1]
		local = netip.AddrFrom4([4]byte(pkt[12:16]))
		peer = netip.AddrFrom4([4]byte(pkt[16:20]))
		udp = pkt[ihl:total]
	case 6:
		if len(pkt) < ipv6HeaderLen+udpHeaderLen || pkt[6] != ipProtoUDP {
			return local, peer, 0, nil, false
		}

		total = ipv6HeaderLen + int(binary.BigEndian.Uint16(pkt[4:6]))
		if total > len(pkt) || total < ipv6HeaderLen+udpHeaderLen {
			return local, peer, 0, nil, false
		}

		tclass = pkt[0]<<4 | pkt[1]>>4
		local = netip.AddrFrom16([16]byte(pkt[8:24]))
		peer = netip.AddrFrom16([16]byte(pkt[24:40]))
		udp = pkt[ipv6HeaderLen:total]
	default:
		return local, peer, 0, nil, false
	}

	if binary.BigEndian.Uint16(udp[2:4]) != gtpuPort {
		return local, peer, 0, nil, false
	}

	return local, peer, tclass, udp[udpHeaderLen:], true
}

func registerFragmentationMetrics() {
	prometheus.MustRegister(gtpuFragmentation)

	for _, r := range []string{reasmReassembled, reasmIgnored, reasmUnknownTEID, reasmTooLarge, reasmInjectFailed} {
		gtpuFragmentation.WithLabelValues(directionUplink, r)
	}

	for _, r := range []string{fragTxSent, fragTxSendFailed, fragTxMalformed} {
		gtpuFragmentation.WithLabelValues(directionDownlink, r)
	}
}
