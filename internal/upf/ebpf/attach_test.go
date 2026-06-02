// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"context"
	"net"
	"os/exec"
	"testing"
	"time"

	"github.com/cilium/ebpf/link"
	"golang.org/x/sys/unix"
)

func ipLink(args ...string) ([]byte, error) {
	return exec.CommandContext(context.Background(), "ip", append([]string{"link"}, args...)...).CombinedOutput()
}

// Real-attach (T2) harness: create a veth pair, attach the program to one end in
// generic XDP mode, and inject frames on the peer so they are received (and run
// through XDP) on the attached end. Unlike BPF_PROG_TEST_RUN this exercises the
// real RX path, with the attached interface's real ifindex as ingress_ifindex.

const (
	t2VethRx = "ellt2rx" // program attaches here; ingress_ifindex of injected frames
	t2VethTx = "ellt2tx" // peer; frames sent here arrive on t2VethRx
)

// attachProgram brings up a veth pair, loads+configures the program via loader
// (passed the attached interface's ifindex), attaches it to that interface, and
// returns the objects plus an inject function. All resources are torn down via
// t.Cleanup.
func attachProgram(t *testing.T, loader func(rxIfindex int) *BpfObjects) (*BpfObjects, func(frame []byte)) {
	t.Helper()

	_, _ = ipLink("del", t2VethRx)

	if out, err := ipLink("add", t2VethRx, "type", "veth", "peer", "name", t2VethTx); err != nil {
		t.Fatalf("create veth pair: %v: %s", err, out)
	}

	t.Cleanup(func() { _, _ = ipLink("del", t2VethRx) })

	for _, dev := range []string{t2VethRx, t2VethTx} {
		if out, err := ipLink("set", dev, "up"); err != nil {
			t.Fatalf("set %s up: %v: %s", dev, err, out)
		}
	}

	rx, err := net.InterfaceByName(t2VethRx)
	if err != nil {
		t.Fatalf("lookup %s: %v", t2VethRx, err)
	}

	tx, err := net.InterfaceByName(t2VethTx)
	if err != nil {
		t.Fatalf("lookup %s: %v", t2VethTx, err)
	}

	obj := loader(rx.Index)

	l, err := link.AttachXDP(link.XDPOptions{
		Program:   obj.UpfN3N6EntrypointFunc,
		Interface: rx.Index,
		Flags:     link.XDPGenericMode,
	})
	if err != nil {
		t.Fatalf("attach XDP to %s: %v", t2VethRx, err)
	}

	t.Cleanup(func() { _ = l.Close() })

	fd, err := unix.Socket(unix.AF_PACKET, unix.SOCK_RAW, 0)
	if err != nil {
		t.Fatalf("AF_PACKET socket: %v", err)
	}

	t.Cleanup(func() { _ = unix.Close(fd) })

	addr := &unix.SockaddrLinklayer{Ifindex: tx.Index, Halen: 6}
	copy(addr.Addr[:6], []byte{0x02, 0, 0, 0, 0, 0x02})

	inject := func(frame []byte) {
		if err := unix.Sendto(fd, frame, 0, addr); err != nil {
			t.Fatalf("inject frame: %v", err)
		}
	}

	return obj, inject
}

// TestDownlinkStatisticsAttached checks that downlink byte accounting lands in
// downlink_statistics when the program is really attached. This is the
// counterpart to TestUplinkStatistics for the N6 side, which the test-run
// harness cannot reach (it needs a distinct ingress interface != the n3 MTU
// device).
func TestDownlinkStatisticsAttached(t *testing.T) {
	requireProgTestRun(t)

	ueIP := [4]byte{10, 45, 0, 2}

	const packets = 5

	obj, inject := attachProgram(t, func(rxIfindex int) *BpfObjects {
		// n6_ifindex == the attached interface, so injected frames (ingress ==
		// that ifindex) are classified N6; n3_ifindex == 1 (loopback) is the
		// encap MTU/egress device.
		o := loadProgramConfig(t, false, false, 1, rxIfindex, 0, 0)
		putDownlinkPDR(t, o, ueIP, 0x42, [4]byte{192, 168, 100, 1}, [4]byte{192, 168, 100, 9}, 5)

		return o
	})

	frame := ethFrame(0x0800, ipv4Packet([4]byte{8, 8, 8, 8}, ueIP, 17, udpDatagram(4000, 4001, nil)))
	for i := 0; i < packets; i++ {
		inject(frame)
	}

	time.Sleep(300 * time.Millisecond)

	dlBytes, _ := sumStats(t, obj.DownlinkStatistics)
	if want := uint64(packets * len(frame)); dlBytes != want {
		t.Errorf("downlink byte_counter = %d, want %d", dlBytes, want)
	}

	if ulBytes, _ := sumStats(t, obj.UplinkStatistics); ulBytes != 0 {
		t.Errorf("uplink byte_counter = %d, want 0", ulBytes)
	}
}
