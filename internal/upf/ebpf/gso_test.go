// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"context"
	"encoding/binary"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"golang.org/x/sys/unix"
)

// A downlink flow large enough to be carried as one GSO super-frame, split so
// the segments are well under the 1500-byte MTU once encapsulated.
const (
	gsoSegmentSize = 1000
	gsoSegments    = 4
)

// gsoFixture is the downlink half of the t2 topology with an IPv6 N3
// transport: the datapath encapsulates toward an IPv6 outer header, and the
// N3 device segments in software on transmit so the wire carries the
// individual segments rather than the super-frame.
type gsoFixture struct {
	obj    *BpfObjects
	n3Peer *net.Interface
	n6Peer *net.Interface
	srcIP  netip.Addr
}

func ethtoolOff(t *testing.T, dev string, features ...string) {
	t.Helper()

	args := append([]string{"-K", dev}, features...)
	if out, err := exec.CommandContext(context.Background(), "ethtool", args...).CombinedOutput(); err != nil {
		t.Fatalf("ethtool -K %s %v: %v: %s", dev, features, err, out)
	}
}

func setupGSO(t *testing.T, senderGSO bool) *gsoFixture {
	t.Helper()

	const (
		n3Dev  = "ellgso3"
		n3Peer = "ellgso3p"
		n6Dev  = "ellgso6"
		n6Peer = "ellgso6p"
	)

	_, _ = ipCmd("link", "del", n3Dev)
	_, _ = ipCmd("link", "del", n6Dev)

	addVethPair(t, n3Dev, n3Peer)
	addVethPair(t, n6Dev, n6Peer)

	if err := writeSysctl("net.ipv4.ip_forward", "1"); err != nil {
		t.Fatalf("enable ip_forward: %v", err)
	}

	// The outer header is IPv6, so the datapath's FIB lookup for it is a
	// forwarding lookup.
	if err := writeSysctl("net.ipv6.conf.all.forwarding", "1"); err != nil {
		t.Fatalf("enable ipv6 forwarding: %v", err)
	}

	upfV6 := netip.AddrFrom16(testUPFN3v6)
	gnbV6 := netip.AddrFrom16(testGNBv6)

	if out, err := ipCmd("addr", "add", upfV6.String()+"/64", "dev", n3Dev, "nodad"); err != nil {
		t.Fatalf("add N3 IPv6 addr: %v: %s", err, out)
	}

	if out, err := ipCmd("neigh", "add", gnbV6.String(), "dev", n3Dev,
		"lladdr", "02:00:00:00:00:bb", "nud", "permanent"); err != nil {
		t.Fatalf("add N3 IPv6 neigh: %v: %s", err, out)
	}

	addAddr(t, n3Dev, addrCIDR(testUPFN3IP))
	addNeigh(t, n3Dev, testGNBIP, "02:00:00:00:00:aa")

	// Segment on egress so the capture sees what a NIC without tunnel
	// checksum offload would put on the wire.
	ethtoolOff(t, n3Dev, "tso", "off", "gso", "off", "tx", "off")

	// N6 side: the sender lives on the peer and routes to the UE through the
	// datapath's N6 device.
	srcIP := netip.AddrFrom4([4]byte{192, 0, 2, 9})

	addAddr(t, n6Dev, addrCIDR(natPublicIP))
	addAddr(t, n6Peer, srcIP.String()+"/24")

	ueAddr := netip.AddrFrom4(ueIP)
	if out, err := ipCmd("route", "add", ueAddr.String()+"/32", "dev", n6Peer); err != nil {
		t.Fatalf("add UE route: %v: %s", err, out)
	}

	n6DevIface := ifByName(t, n6Dev)
	if out, err := ipCmd("neigh", "add", ueAddr.String(), "dev", n6Peer,
		"lladdr", n6DevIface.HardwareAddr.String(), "nud", "permanent"); err != nil {
		t.Fatalf("add UE neigh: %v: %s", err, out)
	}

	if !senderGSO {
		// The mitigation, modelled at its effect: encapsulation only ever
		// sees frames at or below the MTU. Disabling GRO on the N6 receive
		// interface achieves the same thing for merged inbound traffic.
		ethtoolOff(t, n6Peer, "tso", "off", "gso", "off")
	}

	obj := NewBpfObjects(false, false, ifByName(t, n3Dev).Index, n6DevIface.Index, 0, 0)
	obj.UseTCX = true

	if err := obj.Load(); err != nil {
		t.Fatalf("load TC objects: %v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	for _, ifindex := range []int{ifByName(t, n3Dev).Index, n6DevIface.Index} {
		l, err := link.AttachTCX(link.TCXOptions{
			Program:   obj.UpfEntryFunc,
			Attach:    cebpf.AttachTCXIngress,
			Interface: ifindex,
		})
		if err != nil {
			t.Fatalf("attach TCX to ifindex %d: %v", ifindex, err)
		}

		t.Cleanup(func() { _ = l.Close() })
	}

	return &gsoFixture{
		obj:    obj,
		n3Peer: ifByName(t, n3Peer),
		n6Peer: ifByName(t, n6Peer),
		srcIP:  srcIP,
	}
}

// sendSegmented sends one datagram the kernel hands to the datapath as a
// single GSO super-frame, using UDP segmentation offload.
func sendSegmented(t *testing.T, f *gsoFixture, segmented bool) {
	t.Helper()

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		t.Fatalf("socket: %v", err)
	}

	defer func() { _ = unix.Close(fd) }()

	if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: f.srcIP.As4()}); err != nil {
		t.Fatalf("bind: %v", err)
	}

	if segmented {
		if err := unix.SetsockoptInt(fd, unix.IPPROTO_UDP, unix.UDP_SEGMENT, gsoSegmentSize); err != nil {
			t.Fatalf("set UDP_SEGMENT: %v", err)
		}
	}

	payload := make([]byte, gsoSegmentSize*gsoSegments)
	for i := range payload {
		payload[i] = byte(i)
	}

	if err := unix.Sendto(fd, payload, 0, &unix.SockaddrInet4{Addr: ueIP, Port: 4444}); err != nil {
		t.Fatalf("sendto: %v", err)
	}
}

func captureAll(fd int, d time.Duration, match func([]byte) bool) [][]byte {
	var out [][]byte

	deadline := time.Now().Add(d)
	buf := make([]byte, 65536)

	for time.Now().Before(deadline) {
		_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO,
			&unix.Timeval{Usec: 200000})

		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil || n <= 0 {
			continue
		}

		frame := make([]byte, n)
		copy(frame, buf[:n])

		if match(frame) {
			out = append(out, frame)
		}
	}

	return out
}

func isGTPv6Outer(fr []byte) bool {
	return len(fr) >= ethHdrLen+gtpV6EncapLen && fr[12] == 0x86 && fr[13] == 0xDD &&
		fr[ethHdrLen+6] == 17 &&
		binary.BigEndian.Uint16(fr[ethHdrLen+40+2:ethHdrLen+40+4]) == GTPUDPPort
}

// TestTCXIPv6OuterGSOChecksums measures the exposure TCX introduces for an
// IPv6 GTP-U transport: the datapath runs after the kernel has merged inbound
// traffic, so encapsulation can be applied to a frame larger than the MTU.
// Segmentation then recomputes the outer IP and UDP lengths per segment but
// not the outer UDP checksum, which IPv6 requires. The test asserts the
// exposure was actually reached before drawing any conclusion from it.
func TestTCXIPv6OuterGSOChecksums(t *testing.T) {
	requireProgTestRun(t)

	if !testAttachModeTCX() {
		t.Skip("GSO super-frames reach the datapath only at the TC hook; native XDP runs before GRO")
	}

	// __skb_udp_tunnel_segment replays the outer header span byte-for-byte
	// into every segment and fixes up only uh->len, so the GTP-U Length and
	// the IPv6 outer UDP checksum are those of the super-frame. The handling
	// this asserts is not settled: TS 29.281 §5.1 makes Length normative,
	// and refusing the frame costs bulk downlink whenever GRO is on.
	t.Skip("pending the GSO encapsulation decision")

	const (
		teid = 0x6750534F
		qfi  = 7
	)

	f := setupGSO(t, true)
	putDownlinkPDRv6Outer(t, f.obj, ueIP, teid, testUPFN3v6, testGNBv6, qfi)

	capFD := openCapture(t, f.n3Peer.Index)

	sendSegmented(t, f, true)

	all := captureAll(capFD, 2*time.Second, func([]byte) bool { return true })
	for i, fr := range all {
		t.Logf("frame %d: len=%d ethertype=%02x%02x nexthdr=%d gtp=%v",
			i, len(fr), fr[12], fr[13],
			func() int {
				if len(fr) > ethHdrLen+6 {
					return int(fr[ethHdrLen+6])
				}

				return -1
			}(),
			isGTPv6Outer(fr))
	}

	frames := all[:0]
	for _, fr := range all {
		if isGTPv6Outer(fr) {
			frames = append(frames, fr)
		}
	}

	rs := GetN6RouteStats(f.obj)
	t.Logf("verdicts: drop=%d aborted=%d tx=%d redirect=%d pass=%d",
		GetN6Drop(f.obj), GetN6Aborted(f.obj), GetN6Tx(f.obj),
		GetN6Redirect(f.obj), GetN6Pass(f.obj))
	t.Logf("route: success=%d no_neigh=%d mismatch=%d",
		rs.FibSuccess, rs.FibNoNeigh, rs.IfindexMismatch)

	gsoEncaps := GetN6EncapGSOFrames(f.obj)
	t.Logf("captured %d encapsulated segments, encap_gso_frames=%d", len(frames), gsoEncaps)

	if len(frames) == 0 {
		all := captureAll(capFD, 300*time.Millisecond, func([]byte) bool { return true })
		t.Logf("unfiltered frames on N3 peer: %d", len(all))

		for _, d := range []string{"ellgso3", "ellgso6"} {
			for _, k := range []string{"tx_dropped", "tx_errors", "tx_packets", "rx_packets"} {
				b, _ := os.ReadFile("/sys/class/net/" + d + "/statistics/" + k)
				t.Logf("%s %s = %s", d, k, strings.TrimSpace(string(b)))
			}
		}
	}

	if gsoEncaps == 0 {
		t.Skip("encapsulation never saw a GSO super-frame; the exposure was not reached")
	}

	if len(frames) < 2 {
		t.Fatalf("captured %d segments, want the super-frame split into several", len(frames))
	}

	bad := 0

	for i, fr := range frames {
		parsed := parseGTPv6Frame(t, fr)
		if !parsed.udpChecksumOK {
			bad++

			t.Logf("segment %d: outer UDP checksum invalid", i)
		}
	}

	if bad > 0 {
		t.Errorf("%d of %d segments carry an invalid outer UDP checksum", bad, len(frames))
	}
}

// TestTCXIPv6OuterWithoutGSOChecksums is the control: with segmentation
// offload off on the sender, encapsulation only ever sees frames at or below
// the MTU, which is what disabling GRO on the N6 interface achieves for
// merged inbound traffic. Every outer UDP checksum must then be valid.
func TestTCXIPv6OuterWithoutGSOChecksums(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x6750534E
		qfi  = 7
	)

	f := setupGSO(t, false)
	putDownlinkPDRv6Outer(t, f.obj, ueIP, teid, testUPFN3v6, testGNBv6, qfi)

	capFD := openCapture(t, f.n3Peer.Index)

	sendSegmented(t, f, false)

	frames := captureAll(capFD, 2*time.Second, isGTPv6Outer)
	if len(frames) == 0 {
		t.Fatal("captured no encapsulated frames on the N3 side")
	}

	if got := GetN6EncapGSOFrames(f.obj); got != 0 {
		t.Errorf("encap_gso_frames = %d, want 0 with segmentation offload disabled", got)
	}

	for i, fr := range frames {
		if parsed := parseGTPv6Frame(t, fr); !parsed.udpChecksumOK {
			t.Errorf("frame %d: outer UDP checksum invalid without GSO", i)
		}
	}
}

// TestTCXIPv4OuterGSOChecksums is the IPv4-outer counterpart: a zero UDP
// checksum is legal over IPv4, so segmenting an encapsulated super-frame is
// expected to produce valid traffic. It also separates an IPv6-specific
// failure from one inherent to encapsulating a GSO frame.
func TestTCXIPv4OuterGSOChecksums(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x67503449
		qfi  = 7
	)

	f := setupGSO(t, true)
	putDownlinkPDR(t, f.obj, ueIP, teid, testUPFN3IP, testGNBIP, qfi)

	capFD := openCapture(t, f.n3Peer.Index)

	sendSegmented(t, f, true)

	frames := captureAll(capFD, 2*time.Second, func(fr []byte) bool {
		return isGTPv4Outer(fr)
	})

	t.Logf("captured %d encapsulated segments, encap_gso_frames=%d, drop=%d",
		len(frames), GetN6EncapGSOFrames(f.obj), GetN6Drop(f.obj))
}
