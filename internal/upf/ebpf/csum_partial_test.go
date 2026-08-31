// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"testing"

	"golang.org/x/sys/unix"
)

func sendWithIPOptions(t *testing.T, f *gsoFixture, size int) {
	t.Helper()

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		t.Fatalf("socket: %v", err)
	}

	defer func() { _ = unix.Close(fd) }()

	if err := unix.SetsockoptString(fd, unix.IPPROTO_IP, unix.IP_OPTIONS,
		string([]byte{1, 1, 1, 0})); err != nil {
		t.Skipf("IP_OPTIONS unsupported here: %v", err)
	}

	if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: f.srcIP.As4()}); err != nil {
		t.Fatalf("bind: %v", err)
	}

	if err := unix.Sendto(fd, make([]byte, size), 0,
		&unix.SockaddrInet4{Addr: ueIP, Port: 4444}); err != nil {
		t.Fatalf("sendto: %v", err)
	}
}

func TestTCXIPv6OuterInnerIPOptions(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x4F505453
		qfi  = 7
	)

	f := setupGSO(t, false)
	putDownlinkPDRv6Outer(t, f.obj, ueIP, teid, testUPFN3v6, testGNBv6, qfi)

	capFD := openCapture(t, f.n3Peer.Index)

	sendWithIPOptions(t, f, 1000)

	frames := captureAll(capFD, isGTPv6Outer)
	if len(frames) == 0 {
		t.Fatal("captured no encapsulated frames on the N3 side")
	}

	for i, fr := range frames {
		parsed := parseGTPv6Frame(t, fr)
		if !parsed.udpChecksumOK {
			t.Errorf("frame %d: outer UDP checksum invalid", i)
		}

		if inner := gtpInner(fr); len(inner) >= 20 && inner[0]&0x0f == 5 {
			t.Errorf("frame %d: inner ihl is 5, the options were not applied", i)
		}
	}
}

func TestTCXIPv6OuterInnerICMPv4(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid = 0x49434D50
		qfi  = 7
	)

	f := setupGSO(t, false)
	putDownlinkPDRv6Outer(t, f.obj, ueIP, teid, testUPFN3v6, testGNBv6, qfi)

	capFD := openCapture(t, f.n3Peer.Index)

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_ICMP)
	if err != nil {
		t.Skipf("ICMP datagram socket unavailable: %v", err)
	}

	defer func() { _ = unix.Close(fd) }()

	if err := unix.Bind(fd, &unix.SockaddrInet4{Addr: f.srcIP.As4()}); err != nil {
		t.Fatalf("bind: %v", err)
	}

	echo := make([]byte, 64)
	echo[0] = 8

	if err := unix.Sendto(fd, echo, 0, &unix.SockaddrInet4{Addr: ueIP}); err != nil {
		t.Fatalf("sendto: %v", err)
	}

	frames := captureAll(capFD, isGTPv6Outer)
	if len(frames) == 0 {
		t.Fatal("captured no encapsulated frames on the N3 side")
	}

	for i, fr := range frames {
		if parsed := parseGTPv6Frame(t, fr); !parsed.udpChecksumOK {
			t.Errorf("frame %d: outer UDP checksum invalid", i)
		}

		if inner := gtpInner(fr); len(inner) >= 20 && inner[9] != 1 {
			t.Errorf("frame %d: inner proto %d, want ICMP", i, inner[9])
		}
	}
}
