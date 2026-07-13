// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cilium/ebpf/link"
	"golang.org/x/sys/unix"
)

// Real-attach (T2) test fixture.
//
// Unlike BPF_PROG_TEST_RUN (T1), these tests attach the program to real
// interfaces and drive it with packets injected over AF_PACKET, so the program
// runs on the kernel RX path with a real ingress_ifindex and its egress
// (XDP_REDIRECT) actually leaves the interface. This is what lets us observe
// FIB routing and the NAT rewrites on the wire.
//
// The fixture models the UPF's two sides with two veth pairs:
//
//	uplink:   inject on n3Peer ─▶ n3Dev (N3, ingress) ─▶ [XDP] ─▶ redirect ─▶ n6Dev ─▶ n6Peer (capture)
//	downlink: inject on n6Peer ─▶ n6Dev (N6, ingress) ─▶ [XDP] ─▶ redirect ─▶ n3Dev ─▶ n3Peer (capture)
//
// n3_ifindex and n6_ifindex are the real device indices, so the entrypoint
// classifies a packet by the interface it arrives on. The N6 device carries the
// source-NAT egress address and a route to the test server; the N3 device
// carries the UPF's N3 address and a neighbor for the gNB, so bpf_fib_lookup
// resolves deterministically in both directions.

const (
	t2N3Dev  = "ellt2n3"
	t2N3Peer = "ellt2n3p"
	t2N6Dev  = "ellt2n6"
	t2N6Peer = "ellt2n6p"

	t2VethMTU = "3000" // headroom for the payload sweep plus GTP encapsulation
)

var (
	ueIP        = [4]byte{10, 45, 0, 1}     // inner UE address
	natPublicIP = [4]byte{192, 0, 2, 1}     // N6 egress address; source-NAT target
	serverIP    = [4]byte{198, 51, 100, 50} // remote server (RFC 5737 TEST-NET-2)
)

func htons(v uint16) uint16 { return v<<8 | v>>8 }

func ipCmd(args ...string) ([]byte, error) {
	return exec.CommandContext(context.Background(), "ip", args...).CombinedOutput()
}

// t2 is a running real-attach fixture: two veth pairs with the program attached
// to the N3 and N6 devices.
type t2 struct {
	obj    *BpfObjects
	n3Dev  *net.Interface
	n3Peer *net.Interface
	n6Dev  *net.Interface
	n6Peer *net.Interface
}

// setupT2 builds the topology, configures routing, loads the program with the
// given NAT setting, and attaches it to both the N3 and N6 devices.
func setupT2(t *testing.T, masquerade bool) *t2 {
	t.Helper()

	_, _ = ipCmd("link", "del", t2N3Dev)
	_, _ = ipCmd("link", "del", t2N6Dev)

	addVethPair(t, t2N3Dev, t2N3Peer)
	addVethPair(t, t2N6Dev, t2N6Peer)

	if err := writeSysctl("net.ipv4.ip_forward", "1"); err != nil {
		t.Fatalf("enable ip_forward: %v", err)
	}

	// N3 side: the UPF's N3 address and a neighbor for the gNB, so a
	// re-encapsulated downlink packet routes out toward the gNB.
	addAddr(t, t2N3Dev, addrCIDR(testUPFN3IP, 24))
	addNeigh(t, t2N3Dev, testGNBIP, "02:00:00:00:00:aa")

	// N6 side: the source-NAT egress address, a route to the server, and a
	// neighbor, so an uplink packet routes out with src = natPublicIP.
	addAddr(t, t2N6Dev, addrCIDR(natPublicIP, 24))
	addRoute(t, "198.51.100.0/24", t2N6Dev, natPublicIP)
	addNeigh(t, t2N6Dev, serverIP, "02:00:00:00:00:bb")

	f := &t2{
		obj:    loadProgramConfig(t, false, masquerade, ifByName(t, t2N3Dev).Index, ifByName(t, t2N6Dev).Index, 0, 0),
		n3Dev:  ifByName(t, t2N3Dev),
		n3Peer: ifByName(t, t2N3Peer),
		n6Dev:  ifByName(t, t2N6Dev),
		n6Peer: ifByName(t, t2N6Peer),
	}

	attachXDP(t, f.obj, f.n3Dev.Index)
	attachXDP(t, f.obj, f.n6Dev.Index)

	return f
}

func (f *t2) injectUplink(t *testing.T, frame []byte)   { inject(t, f.n3Peer.Index, frame) }
func (f *t2) injectDownlink(t *testing.T, frame []byte) { inject(t, f.n6Peer.Index, frame) }

// captureN6 and captureN3 capture frames leaving the program on each side
// (redirected, or reflected via XDP_TX). They are named by physical side, not
// direction: an uplink packet egresses on N6, a downlink packet on N3, but a
// downlink that triggers an ICMP error reflects back out N6.
func (f *t2) captureN6(t *testing.T) int { return openCapture(t, f.n6Peer.Index) }
func (f *t2) captureN3(t *testing.T) int { return openCapture(t, f.n3Peer.Index) }

func addVethPair(t *testing.T, dev, peer string) {
	t.Helper()

	if out, err := ipCmd("link", "add", dev, "type", "veth", "peer", "name", peer); err != nil {
		t.Fatalf("create veth %s/%s: %v: %s", dev, peer, err, out)
	}

	t.Cleanup(func() { _, _ = ipCmd("link", "del", dev) })

	for _, d := range []string{dev, peer} {
		if out, err := ipCmd("link", "set", d, "mtu", t2VethMTU, "up"); err != nil {
			t.Fatalf("set %s up: %v: %s", d, err, out)
		}

		_ = writeSysctl("net.ipv4.conf."+d+".forwarding", "1")
	}
}

func addAddr(t *testing.T, dev, cidr string) {
	t.Helper()

	if out, err := ipCmd("addr", "add", cidr, "dev", dev); err != nil {
		t.Fatalf("add addr %s on %s: %v: %s", cidr, dev, err, out)
	}
}

func addRoute(t *testing.T, prefix, dev string, src [4]byte) {
	t.Helper()

	if out, err := ipCmd("route", "add", prefix, "dev", dev, "src", ip4String(src)); err != nil {
		t.Fatalf("add route %s: %v: %s", prefix, err, out)
	}
}

func addNeigh(t *testing.T, dev string, addr [4]byte, lladdr string) {
	t.Helper()

	if out, err := ipCmd("neigh", "add", ip4String(addr), "dev", dev, "lladdr", lladdr, "nud", "permanent"); err != nil {
		t.Fatalf("add neigh %s: %v: %s", ip4String(addr), err, out)
	}
}

func attachXDP(t *testing.T, obj *BpfObjects, ifindex int) {
	t.Helper()

	l, err := link.AttachXDP(link.XDPOptions{
		Program:   obj.UpfEntryFunc,
		Interface: ifindex,
		Flags:     link.XDPGenericMode,
	})
	if err != nil {
		t.Fatalf("attach XDP to ifindex %d: %v", ifindex, err)
	}

	t.Cleanup(func() { _ = l.Close() })
}

func inject(t *testing.T, ifindex int, frame []byte) {
	t.Helper()

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, 0)
	if err != nil {
		t.Fatalf("AF_PACKET inject socket: %v", err)
	}

	defer func() { _ = unix.Close(fd) }()

	addr := &unix.SockaddrLinklayer{Ifindex: ifindex, Halen: 6}
	copy(addr.Addr[:6], []byte{0x02, 0, 0, 0, 0, 0x02})

	if err := unix.Sendto(fd, frame, 0, addr); err != nil {
		t.Fatalf("inject frame on ifindex %d: %v", ifindex, err)
	}
}

func openCapture(t *testing.T, ifindex int) int {
	t.Helper()

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, int(htons(unix.ETH_P_ALL)))
	if err != nil {
		t.Fatalf("AF_PACKET capture socket: %v", err)
	}

	t.Cleanup(func() { _ = unix.Close(fd) })

	if err := unix.Bind(fd, &unix.SockaddrLinklayer{Protocol: htons(unix.ETH_P_ALL), Ifindex: ifindex}); err != nil {
		t.Fatalf("bind capture: %v", err)
	}

	_ = unix.SetsockoptTimeval(fd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix.Timeval{Usec: 200_000})

	return fd
}

// captureMatching reads frames until match returns true or the timeout elapses.
func captureMatching(fd int, timeout time.Duration, match func([]byte) bool) []byte { //nolint:unparam // general helper; timeout is configurable
	buf := make([]byte, 9000)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		n, _, err := unix.Recvfrom(fd, buf, 0)
		if err != nil {
			continue // RCVTIMEO slice elapsed; retry until the overall deadline
		}

		frame := make([]byte, n)
		copy(frame, buf[:n])

		if match(frame) {
			return frame
		}
	}

	return nil
}

func writeSysctl(key, value string) error { //nolint:unparam // general helper; value is configurable
	return os.WriteFile("/proc/sys/"+strings.ReplaceAll(key, ".", "/"), []byte(value), 0o644)
}

func ifByName(t *testing.T, name string) *net.Interface {
	t.Helper()

	iface, err := net.InterfaceByName(name)
	if err != nil {
		t.Fatalf("lookup %s: %v", name, err)
	}

	return iface
}

func ip4String(a [4]byte) string { return net.IP(a[:]).String() }

func addrCIDR(a [4]byte, prefix int) string { return ip4String(a) + "/" + strconv.Itoa(prefix) }

// TestDownlinkStatisticsAttached checks that downlink byte accounting lands in
// downlink_statistics on a real attach. It is the N6-side counterpart to
// TestUplinkStatistics, which the test-run harness cannot reach because it needs
// a distinct ingress interface.
func TestDownlinkStatisticsAttached(t *testing.T) {
	requireProgTestRun(t)

	const packets = 5

	f := setupT2(t, false)
	putDownlinkPDR(t, f.obj, ueIP, 0x42, testUPFN3IP, testGNBIP, 5)

	frame := ethFrame(0x0800, ipv4Packet(serverIP, ueIP, 17, udpDatagram(4000, 4001, nil)))
	for i := 0; i < packets; i++ {
		f.injectDownlink(t, frame)
	}

	time.Sleep(300 * time.Millisecond)

	dlBytes, _ := sumStats(t, f.obj.DownlinkStatistics)
	if want := uint64(packets * len(frame)); dlBytes != want {
		t.Errorf("downlink byte_counter = %d, want %d", dlBytes, want)
	}

	if ulBytes, _ := sumStats(t, f.obj.UplinkStatistics); ulBytes != 0 {
		t.Errorf("uplink byte_counter = %d, want 0", ulBytes)
	}
}
