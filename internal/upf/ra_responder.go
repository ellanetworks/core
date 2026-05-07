// Copyright 2026 Ella Networks

package upf

import (
	"bytes"
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
	"github.com/ellanetworks/core/internal/ndp"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// IPv6SessionContext holds the tunnel metadata and prefix information
// needed to respond to an RS from a UE with an RA.
type IPv6SessionContext struct {
	DownlinkTEID uint32     // gNB's TEID for DL GTP-U encapsulation
	GnbN3Addr    netip.Addr // gNB's N3 transport address (IPv4 or IPv6)
	Prefix       netip.Prefix
	MTU          uint32
	QFI          uint8

	// ueLinkLocal is learned from the first RS event. Once set, the
	// corresponding veth tunnel map entry exists and is cleaned up on
	// session release.
	ueLinkLocal netip.Addr
}

// routerLinkLocal is the link-local address used as the RA source. We use a
// well-known address (fe80::1) because the GTP tunnel is point-to-point and
// the UE does not need to discover this address.
var routerLinkLocal = netip.MustParseAddr("fe80::1")

// RAResponder listens for IPv6 Router Solicitation events from the N3 XDP
// ring buffer and responds with Router Advertisements via the veth injection
// path. It manages:
//   - An in-memory map of UL TEID → IPv6SessionContext
//   - The veth-smf / veth-xdp pair and veth XDP program
//   - RA packet construction and injection via AF_PACKET
type RAResponder struct {
	mu       sync.RWMutex
	sessions map[uint32]*IPv6SessionContext // key: uplink TEID

	// N3 interface info (needed for veth tunnel map programming).
	n3IPv4    netip.Addr
	n3IPv6    netip.Addr
	n3Ifindex int

	// Veth XDP objects.
	vethBpf  *ebpf.VethBpfObjects
	vethLink link.Link

	// RS ring buffer reader (from N3 XDP rs_event_map).
	rsReader *ringbuf.Reader

	// AF_PACKET injection socket on veth-smf.
	injectFD    int
	injectSA    unix.SockaddrLinklayer
	vethSmfMAC  [6]byte
	vethXdpMAC  [6]byte
	vethSmfName string
}

// NewRAResponder creates an RA responder. It does not start the consumer
// goroutine; call Start() to begin processing.
func NewRAResponder(rsEventMap *bpf.Map, n3IPv4 netip.Addr, n3IPv6 netip.Addr, n3Ifindex int) (*RAResponder, error) {
	rsReader, err := ringbuf.NewReader(rsEventMap)
	if err != nil {
		return nil, fmt.Errorf("open RS event ring buffer: %w", err)
	}

	return &RAResponder{
		sessions:  make(map[uint32]*IPv6SessionContext),
		n3IPv4:    n3IPv4,
		n3IPv6:    n3IPv6,
		n3Ifindex: n3Ifindex,
		rsReader:  rsReader,
		injectFD:  -1,
	}, nil
}

// Start initializes the veth pair, loads the veth XDP program, attaches it
// to veth-xdp, opens the AF_PACKET injection socket, and starts the RS event
// consumer goroutine.
func (r *RAResponder) Start() error {
	// Create the veth pair.
	if err := CreateVethPair(); err != nil {
		return fmt.Errorf("create veth pair: %w", err)
	}

	// Load veth XDP objects.
	vethBpf, err := ebpf.LoadVethBpfObjects()
	if err != nil {
		return fmt.Errorf("load veth XDP program: %w", err)
	}

	r.vethBpf = vethBpf

	// Attach XDP to veth-xdp.
	xdpIdx, err := VethXDPIndex()
	if err != nil {
		return fmt.Errorf("veth-xdp interface not found: %w", err)
	}

	xdpLink, err := link.AttachXDP(link.XDPOptions{
		Program:   vethBpf.VethXdpFunc,
		Interface: xdpIdx,
		Flags:     link.XDPGenericMode,
	})
	if err != nil {
		return fmt.Errorf("attach XDP to veth-xdp: %w", err)
	}

	r.vethLink = xdpLink

	// Resolve veth MACs for Ethernet framing of injected packets.
	smfIface, err := net.InterfaceByName(VethSMFName)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", VethSMFName, err)
	}

	copy(r.vethSmfMAC[:], smfIface.HardwareAddr)
	r.vethSmfName = VethSMFName

	xdpIface, err := net.InterfaceByName(VethXDPName)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", VethXDPName, err)
	}

	copy(r.vethXdpMAC[:], xdpIface.HardwareAddr)

	// Open AF_PACKET socket on veth-smf for RA injection.
	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		return fmt.Errorf("AF_PACKET socket: %w", err)
	}

	r.injectFD = fd
	r.injectSA = unix.SockaddrLinklayer{
		Protocol: htons(unix.ETH_P_ALL),
		Ifindex:  smfIface.Index,
	}

	// Start the RS event consumer goroutine.
	go r.listenForRSEvents() // #nosec: G118 -- lifecycle goroutine

	logger.UpfLog.Info("RA responder started",
		zap.String("veth_smf", VethSMFName),
		zap.String("veth_xdp", VethXDPName),
		zap.String("n3_ipv4", r.n3IPv4.String()),
		zap.String("n3_ipv6", r.n3IPv6.String()),
		zap.Int("n3_ifindex", r.n3Ifindex),
	)

	return nil
}

// Close stops the RS consumer and releases all resources.
func (r *RAResponder) Close() error {
	// Close ring buffer reader first (unblocks the consumer goroutine).
	if r.rsReader != nil {
		_ = r.rsReader.Close()
	}

	if r.vethLink != nil {
		_ = r.vethLink.Close()
	}

	if r.vethBpf != nil {
		_ = r.vethBpf.Close()
	}

	if r.injectFD >= 0 {
		_ = unix.Close(r.injectFD)
		r.injectFD = -1
	}

	if err := DestroyVethPair(); err != nil {
		logger.UpfLog.Warn("failed to destroy veth pair", zap.Error(err))
	}

	return nil
}

// RegisterSession stores the IPv6 session metadata needed for RA responses.
// Called when an IPv6 PDU session is fully established (after N2 handover
// provides the gNB's tunnel endpoint).
func (r *RAResponder) RegisterSession(ulTEID uint32, sessionCtx *IPv6SessionContext) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[ulTEID] = sessionCtx

	logger.UpfLog.Debug("registered IPv6 session for RA",
		logger.TEID(ulTEID),
		zap.String("gnb_n3_addr", sessionCtx.GnbN3Addr.String()),
		zap.String("prefix", sessionCtx.Prefix.String()),
		zap.Uint32("dl_teid", sessionCtx.DownlinkTEID),
	)
}

// UnregisterSession removes a session and cleans up any veth tunnel map
// entry that was created for RA delivery.
func (r *RAResponder) UnregisterSession(ulTEID uint32) {
	r.mu.Lock()

	sess, ok := r.sessions[ulTEID]
	if ok {
		delete(r.sessions, ulTEID)
	}
	r.mu.Unlock()

	if !ok {
		return
	}

	// Clean up the veth tunnel map entry if we programmed one.
	if sess.ueLinkLocal.IsValid() && r.vethBpf != nil {
		if err := r.vethBpf.DeleteTunnel(sess.ueLinkLocal); err != nil {
			logger.UpfLog.Warn("failed to delete veth tunnel entry on session release",
				logger.TEID(ulTEID),
				zap.String("ue_link_local", sess.ueLinkLocal.String()),
				zap.Error(err),
			)
		}
	}

	logger.UpfLog.Debug("unregistered IPv6 session for RA", logger.TEID(ulTEID))
}

// listenForRSEvents is the ring buffer consumer goroutine. It blocks on
// ReadInto and dispatches each RS event to handleRSEvent.
func (r *RAResponder) listenForRSEvents() {
	var record ringbuf.Record

	for {
		err := r.rsReader.ReadInto(&record)
		if errors.Is(err, os.ErrClosed) {
			return
		}

		if err != nil {
			logger.UpfLog.Warn("RS ring buffer read error", zap.Error(err))
			continue
		}

		var event ebpf.RSEvent
		if err := binary.Read(bytes.NewReader(record.RawSample), binary.NativeEndian, &event); err != nil {
			logger.UpfLog.Warn("failed to decode RS event", zap.Error(err))
			continue
		}

		ueIPv6 := netip.AddrFrom16(event.UEIPv6)
		if err := r.handleRSEvent(event.TEID, ueIPv6); err != nil {
			logger.UpfLog.Warn("failed to handle RS event",
				logger.TEID(event.TEID),
				zap.String("ue_ipv6", ueIPv6.String()),
				zap.Error(err),
			)
		}
	}
}

// handleRSEvent processes a single RS event: looks up the session, programs
// the veth tunnel map, builds the RA, and injects it.
func (r *RAResponder) handleRSEvent(ulTEID uint32, ueIPv6 netip.Addr) error {
	r.mu.Lock()

	sess, ok := r.sessions[ulTEID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("no IPv6 session for TEID 0x%X", ulTEID)
	}

	// If the UE changed its link-local address, delete the stale map entry.
	if sess.ueLinkLocal.IsValid() && sess.ueLinkLocal != ueIPv6 {
		_ = r.vethBpf.DeleteTunnel(sess.ueLinkLocal)
	}

	sess.ueLinkLocal = ueIPv6
	r.mu.Unlock()

	// Program the veth tunnel map so the RA packet (with dst=ueIPv6) gets
	// GTP-U encapsulated and redirected to N3.
	if err := r.programVethTunnel(sess, ueIPv6); err != nil {
		return fmt.Errorf("program veth tunnel: %w", err)
	}

	// Build and inject the RA.
	raPkt, err := r.buildRAPacket(sess, ueIPv6)
	if err != nil {
		return fmt.Errorf("build RA: %w", err)
	}

	logger.UpfLog.Info("injecting RA via AF_PACKET",
		zap.String("interface", r.vethSmfName),
		zap.Int("packet_size", len(raPkt)))

	if err := unix.Sendto(r.injectFD, raPkt, 0, &r.injectSA); err != nil {
		return fmt.Errorf("inject RA on %s: %w", r.vethSmfName, err)
	}

	logger.UpfLog.Info("sent RA via veth",
		logger.TEID(ulTEID),
		zap.String("ue_link_local", ueIPv6.String()),
		zap.String("prefix", sess.Prefix.String()),
	)

	return nil
}

// programVethTunnel ensures the veth tunnel map has an entry for the given
// UE IPv6 address. This allows the veth XDP program to encapsulate the RA
// in GTP-U and redirect it to N3.
func (r *RAResponder) programVethTunnel(sess *IPv6SessionContext, ueIPv6 netip.Addr) error {
	localAddr, err := r.localN3Addr(sess.GnbN3Addr)
	if err != nil {
		return err
	}

	info := ebpf.VethTunnelInfo{
		TEID:       sess.DownlinkTEID,
		LocalAddr:  localAddr,
		RemoteAddr: sess.GnbN3Addr,
		QFI:        sess.QFI,
	}

	return r.vethBpf.PutTunnel(ueIPv6, info)
}

func (r *RAResponder) localN3Addr(remote netip.Addr) (netip.Addr, error) {
	if remote.Is4() {
		if r.n3IPv4.IsValid() {
			return r.n3IPv4, nil
		}

		return netip.Addr{}, fmt.Errorf("no local N3 IPv4 address available for remote %s", remote)
	}

	if remote.Is6() {
		if r.n3IPv6.IsValid() {
			return r.n3IPv6, nil
		}

		return netip.Addr{}, fmt.Errorf("no local N3 IPv6 address available for remote %s", remote)
	}

	return netip.Addr{}, fmt.Errorf("invalid gNB N3 address: %s", remote)
}

// buildRAPacket constructs a complete Ethernet + IPv6 + ICMPv6 RA frame
// ready for injection on veth-smf.
func (r *RAResponder) buildRAPacket(sess *IPv6SessionContext, ueIPv6 netip.Addr) ([]byte, error) {
	// Build the ICMPv6 RA body.
	raBody, err := ndp.BuildRA(ndp.RAParams{
		SrcIP:             routerLinkLocal,
		DstIP:             ueIPv6,
		CurHopLimit:       64,
		RouterLifetime:    1800, // 30 minutes
		Prefix:            sess.Prefix,
		OnLink:            true,
		Autonomous:        true,
		ValidLifetime:     0xFFFFFFFF, // infinity
		PreferredLifetime: 0xFFFFFFFF, // infinity
		MTU:               sess.MTU,
	})
	if err != nil {
		return nil, fmt.Errorf("build RA body: %w", err)
	}

	// Compute ICMPv6 checksum.
	ndp.SetICMPv6Checksum(routerLinkLocal, ueIPv6, raBody)

	// Build IPv6 header.
	ipv6Hdr := make([]byte, 40)
	ipv6Hdr[0] = 0x60                                             // version=6
	binary.BigEndian.PutUint16(ipv6Hdr[4:6], uint16(len(raBody))) // payload length
	ipv6Hdr[6] = 58                                               // next header = ICMPv6
	ipv6Hdr[7] = 255                                              // hop limit = 255 (required for NDP)
	src16 := routerLinkLocal.As16()
	copy(ipv6Hdr[8:24], src16[:])

	dst16 := ueIPv6.As16()
	copy(ipv6Hdr[24:40], dst16[:])

	// Build Ethernet header (for veth-smf → veth-xdp).
	eth := make([]byte, 14)
	copy(eth[0:6], r.vethXdpMAC[:])                // dst MAC = veth-xdp
	copy(eth[6:12], r.vethSmfMAC[:])               // src MAC = veth-smf
	binary.BigEndian.PutUint16(eth[12:14], 0x86DD) // EtherType = IPv6

	// Assemble the complete frame.
	pkt := make([]byte, 0, len(eth)+len(ipv6Hdr)+len(raBody))
	pkt = append(pkt, eth...)
	pkt = append(pkt, ipv6Hdr...)
	pkt = append(pkt, raBody...)

	return pkt, nil
}

// htons converts a uint16 from host to network byte order.
func htons(v uint16) uint16 {
	var buf [2]byte
	binary.NativeEndian.PutUint16(buf[:], v)

	return binary.BigEndian.Uint16(buf[:])
}
